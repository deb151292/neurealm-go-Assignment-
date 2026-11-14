package tenant

import (
	"context"
	"strings"

	"github.com/deb151292/neurealm-go-Assignment-/internal/audit"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/validator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateTenantRequest struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

type UpdateTenantRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Service struct {
	repo         Repository
	pool         *pgxpool.Pool
	auditService *audit.Service
}

func NewService(repo Repository, pool *pgxpool.Pool, auditService *audit.Service) *Service {
	return &Service{
		repo:         repo,
		pool:         pool,
		auditService: auditService,
	}
}

func (s *Service) CreateTenant(ctx context.Context, req CreateTenantRequest) (*domain.Tenant, error) {
	v := validator.New()
	v.RequiredString(req.Name, "name")
	v.TenantCode(req.Code, "code")
	if req.Status == "" {
		req.Status = domain.TenantStatusActive
	} else {
		v.In(req.Status, "status", domain.TenantStatusActive, domain.TenantStatusSuspended)
	}
	if !v.Valid() {
		return nil, v.ToAppError()
	}

	t := &domain.Tenant{
		Name:   strings.TrimSpace(req.Name),
		Code:   strings.ToLower(strings.TrimSpace(req.Code)),
		Status: req.Status,
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}

	// Bootstrap default system roles for the tenant (Admin and Member)
	if s.pool != nil {
		s.bootstrapTenantRoles(ctx, t.ID)
	}

	// Record audit event
	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &t.ID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "tenant:create",
			TargetType: "tenant",
			TargetID:   t.ID.String(),
			Metadata: map[string]interface{}{
				"code": t.Code,
				"name": t.Name,
			},
		})
	}

	return t, nil
}

func (s *Service) GetTenant(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetTenantByCode(ctx context.Context, code string) (*domain.Tenant, error) {
	return s.repo.GetByCode(ctx, strings.ToLower(strings.TrimSpace(code)))
}

func (s *Service) UpdateTenant(ctx context.Context, id uuid.UUID, req UpdateTenantRequest) (*domain.Tenant, error) {
	v := validator.New()
	v.RequiredString(req.Name, "name")
	v.In(req.Status, "status", domain.TenantStatusActive, domain.TenantStatusSuspended)
	if !v.Valid() {
		return nil, v.ToAppError()
	}

	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	t.Name = strings.TrimSpace(req.Name)
	t.Status = req.Status

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &t.ID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "tenant:update",
			TargetType: "tenant",
			TargetID:   t.ID.String(),
			Metadata: map[string]interface{}{
				"status": t.Status,
				"name":   t.Name,
			},
		})
	}

	return t, nil
}

func (s *Service) ListTenants(ctx context.Context, params domain.PaginationParams) ([]domain.Tenant, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	return s.repo.List(ctx, params)
}

func (s *Service) bootstrapTenantRoles(ctx context.Context, tenantID uuid.UUID) {
	if s.pool == nil {
		return
	}
	// Create Admin role and assign all system permissions
	var adminRoleID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO roles (tenant_id, name, description, is_system)
		VALUES ($1, 'Admin', 'Default tenant administrator with full access', TRUE)
		ON CONFLICT (tenant_id, name) DO UPDATE SET is_system = TRUE
		RETURNING id
	`, tenantID).Scan(&adminRoleID)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions
			ON CONFLICT DO NOTHING
		`, adminRoleID)
	}

	// Create Member role with read permissions
	var memberRoleID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO roles (tenant_id, name, description, is_system)
		VALUES ($1, 'Member', 'Default standard tenant member', TRUE)
		ON CONFLICT (tenant_id, name) DO UPDATE SET is_system = TRUE
		RETURNING id
	`, tenantID).Scan(&memberRoleID)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions
			WHERE (resource = 'user' AND action = 'read') OR (resource = 'tenant' AND action = 'read')
			ON CONFLICT DO NOTHING
		`, memberRoleID)
	}
}
