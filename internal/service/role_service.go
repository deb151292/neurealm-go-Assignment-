package service

import (
	"context"
	"strings"

	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/repository"
	"github.com/deb151292/neurealm-go-Assignment-/internal/validator"
	"github.com/google/uuid"
)

type CreateRoleRequest struct {
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	PermissionIDs []uuid.UUID `json:"permission_ids"`
}

type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AssignPermissionsRequest struct {
	PermissionIDs []uuid.UUID `json:"permission_ids"`
}

type AssignUserRolesRequest struct {
	RoleIDs []uuid.UUID `json:"role_ids"`
}

type RoleService struct {
	repo         repository.RoleRepository
	cache        *cache.PermissionCache
	auditService *AuditService
}

func NewRoleService(repo repository.RoleRepository, cache *cache.PermissionCache, auditService *AuditService) *RoleService {
	return &RoleService{
		repo:         repo,
		cache:        cache,
		auditService: auditService,
	}
}

func (s *RoleService) CreateRole(ctx context.Context, tenantID uuid.UUID, req CreateRoleRequest) (*domain.Role, error) {
	v := validator.New()
	v.RequiredString(req.Name, "name")
	if !v.Valid() {
		return nil, v.ToAppError()
	}

	role := &domain.Role{
		TenantID:    tenantID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		IsSystem:    false,
	}

	if err := s.repo.Create(ctx, role); err != nil {
		return nil, err
	}

	if len(req.PermissionIDs) > 0 {
		_ = s.repo.AssignPermissions(ctx, tenantID, role.ID, req.PermissionIDs)
		perms, _ := s.repo.GetRolePermissions(ctx, role.ID)
		role.Permissions = perms
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &tenantID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "role:create",
			TargetType: "role",
			TargetID:   role.ID.String(),
			Metadata: map[string]interface{}{
				"role_name":   role.Name,
				"permissions": len(req.PermissionIDs),
			},
		})
	}

	return role, nil
}

func (s *RoleService) GetRole(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.Role, error) {
	return s.repo.GetByID(ctx, tenantID, roleID)
}

func (s *RoleService) ListRoles(ctx context.Context, tenantID uuid.UUID, params domain.PaginationParams) ([]domain.Role, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	return s.repo.ListByTenant(ctx, tenantID, params)
}

func (s *RoleService) UpdateRole(ctx context.Context, tenantID, roleID uuid.UUID, req UpdateRoleRequest) (*domain.Role, error) {
	v := validator.New()
	v.RequiredString(req.Name, "name")
	if !v.Valid() {
		return nil, v.ToAppError()
	}

	role, err := s.repo.Update(ctx, tenantID, roleID, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description))
	if err != nil {
		return nil, err
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &tenantID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "role:update",
			TargetType: "role",
			TargetID:   role.ID.String(),
			Metadata: map[string]interface{}{
				"role_name": role.Name,
			},
		})
	}

	return role, nil
}

func (s *RoleService) DeleteRole(ctx context.Context, tenantID, roleID uuid.UUID) error {
	if err := s.repo.Delete(ctx, tenantID, roleID); err != nil {
		return err
	}

	if s.cache != nil {
		s.cache.InvalidateTenant(tenantID)
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &tenantID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "role:delete",
			TargetType: "role",
			TargetID:   roleID.String(),
			Metadata:   map[string]interface{}{},
		})
	}

	return nil
}

func (s *RoleService) AssignPermissions(ctx context.Context, tenantID, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	if err := s.repo.AssignPermissions(ctx, tenantID, roleID, permissionIDs); err != nil {
		return err
	}

	if s.cache != nil {
		s.cache.InvalidateTenant(tenantID)
	}

	actorID, _ := domain.GetUserID(ctx)
	actorEmail := domain.GetUserEmail(ctx)
	if s.auditService != nil {
		s.auditService.RecordAsync(&domain.AuditLog{
			TenantID:   &tenantID,
			ActorID:    &actorID,
			ActorEmail: actorEmail,
			Action:     "role:permissions_update",
			TargetType: "role",
			TargetID:   roleID.String(),
			Metadata: map[string]interface{}{
				"permissions_count": len(permissionIDs),
			},
		})
	}

	return nil
}

func (s *RoleService) AssignUserRoles(ctx context.Context, tenantID, userID uuid.UUID, roleIDs []uuid.UUID) error {
	if err := s.repo.AssignUserRoles(ctx, tenantID, userID, roleIDs); err != nil {
		return err
	}

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
			Action:     "role:assign",
			TargetType: "user",
			TargetID:   userID.String(),
			Metadata: map[string]interface{}{
				"roles_count": len(roleIDs),
			},
		})
	}

	return nil
}

func (s *RoleService) GetUserPermissions(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	if s.cache != nil {
		if perms, found := s.cache.Get(tenantID, userID); found {
			list := make([]string, 0, len(perms))
			for p := range perms {
				list = append(list, p)
			}
			return list, nil
		}
	}

	if s.repo == nil {
		return nil, nil
	}

	perms, err := s.repo.GetUserPermissions(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		s.cache.Set(tenantID, userID, perms)
	}
	return perms, nil
}

func (s *RoleService) GetUserRoles(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.Role, error) {
	return s.repo.GetUserRoles(ctx, tenantID, userID)
}
