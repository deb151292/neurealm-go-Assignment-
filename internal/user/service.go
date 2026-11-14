package user

import (
	"context"
	"strings"

	"github.com/deb151292/neurealm-go-Assignment-/internal/audit"
	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/validator"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type CreateUserRequest struct {
	Email     string      `json:"email"`
	Password  string      `json:"password"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	RoleIDs   []uuid.UUID `json:"role_ids"`
}

type UpdateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Status    string `json:"status"`
}

type Service struct {
	repo         Repository
	cache        *cache.PermissionCache
	auditService *audit.Service
}

func NewService(repo Repository, cache *cache.PermissionCache, auditService *audit.Service) *Service {
	return &Service{
		repo:         repo,
		cache:        cache,
		auditService: auditService,
	}
}

func (s *Service) CreateUser(ctx context.Context, tenantID uuid.UUID, req CreateUserRequest) (*domain.User, error) {
	v := validator.New()
	v.Email(req.Email, "email")
	v.MinLength(req.Password, "password", 8)
	v.RequiredString(req.FirstName, "first_name")
	v.RequiredString(req.LastName, "last_name")
	if !v.Valid() {
		return nil, v.ToAppError()
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		PasswordHash: string(hashedPassword),
		FirstName:    strings.TrimSpace(req.FirstName),
		LastName:     strings.TrimSpace(req.LastName),
	}

	if err := s.repo.CreateUserInTenant(ctx, tenantID, user, req.RoleIDs); err != nil {
		return nil, err
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &tenantID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "user:create",
			TargetType: "user",
			TargetID:   user.ID.String(),
			Metadata: map[string]interface{}{
				"email": user.Email,
			},
		})
	}

	return s.repo.GetByID(ctx, tenantID, user.ID)
}

func (s *Service) GetUser(ctx context.Context, tenantID, userID uuid.UUID) (*domain.User, error) {
	return s.repo.GetByID(ctx, tenantID, userID)
}

func (s *Service) UpdateUser(ctx context.Context, tenantID, userID uuid.UUID, req UpdateUserRequest) (*domain.User, error) {
	v := validator.New()
	v.RequiredString(req.FirstName, "first_name")
	v.RequiredString(req.LastName, "last_name")
	if req.Status != "" {
		v.In(req.Status, "status", domain.UserStatusActive, domain.UserStatusSuspended)
	}
	if !v.Valid() {
		return nil, v.ToAppError()
	}

	user, err := s.repo.Update(ctx, tenantID, userID, strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName), req.Status)
	if err != nil {
		return nil, err
	}

	// Invalidate permission cache
	if s.cache != nil {
		s.cache.InvalidateUser(tenantID, userID)
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &tenantID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "user:update",
			TargetType: "user",
			TargetID:   userID.String(),
			Metadata: map[string]interface{}{
				"status": user.TenantStatus,
			},
		})
	}

	return user, nil
}

func (s *Service) SoftDeleteUser(ctx context.Context, tenantID, userID uuid.UUID) error {
	if err := s.repo.SoftDelete(ctx, tenantID, userID); err != nil {
		return err
	}

	// Invalidate permission cache for the user
	if s.cache != nil {
		s.cache.InvalidateUser(tenantID, userID)
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &tenantID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "user:delete",
			TargetType: "user",
			TargetID:   userID.String(),
			Metadata: map[string]interface{}{
				"soft_delete": true,
			},
		})
	}

	return nil
}

func (s *Service) ListUsers(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.User, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	return s.repo.ListByTenant(ctx, tenantID, params)
}

func (s *Service) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	return s.repo.GetUserTenants(ctx, userID)
}
