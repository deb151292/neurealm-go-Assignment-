package handler

import (
	"github.com/deb151292/neurealm-go-Assignment-/internal/auth"
	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/deb151292/neurealm-go-Assignment-/internal/repository"
	"github.com/deb151292/neurealm-go-Assignment-/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handlers encapsulates all HTTP handlers and their initialized dependencies.
type Handlers struct {
	Auth       *AuthHandler
	Tenant     *TenantHandler
	User       *UserHandler
	Role       *RoleHandler
	Permission *PermissionHandler
	Audit      *AuditHandler

	// Exposed services needed by middlewares (e.g. RBAC)
	RoleService *service.RoleService
	UserService *service.UserService
}

// NewHandlers initializes repositories, services, and handlers by taking pgxpool.Pool,
// propagating the pool down through the layers while keeping server.go clean.
func NewHandlers(pool *pgxpool.Pool, jwtSvc *auth.JWTService, permCache *cache.PermissionCache) *Handlers {
	// 1. Repositories (pool is passed here)
	auditRepo := repository.NewAuditRepository(pool)
	tenantRepo := repository.NewTenantRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	authRepo := repository.NewAuthRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)

	// 2. Services
	auditSvc := service.NewAuditService(auditRepo)
	tenantSvc := service.NewTenantService(tenantRepo, pool, auditSvc)
	roleSvc := service.NewRoleService(roleRepo, permCache, auditSvc)
	userSvc := service.NewUserService(userRepo, permCache, auditSvc)
	permSvc := service.NewPermissionService(permRepo)
	authSvc := service.NewAuthService(authRepo, userRepo, tenantRepo, roleRepo, jwtSvc, permCache, auditSvc)

	// 3. Handlers
	return &Handlers{
		Auth:        NewAuthHandler(authSvc, userSvc, tenantRepo, roleSvc),
		Tenant:      NewTenantHandler(tenantSvc),
		User:        NewUserHandler(userSvc, roleSvc),
		Role:        NewRoleHandler(roleSvc),
		Permission:  NewPermissionHandler(permSvc),
		Audit:       NewAuditHandler(auditSvc),
		RoleService: roleSvc,
		UserService: userSvc,
	}
}
