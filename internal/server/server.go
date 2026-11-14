package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neurealm/multi-tenant-system/internal/audit"
	"github.com/neurealm/multi-tenant-system/internal/auth"
	"github.com/neurealm/multi-tenant-system/internal/cache"
	"github.com/neurealm/multi-tenant-system/internal/config"
	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/middleware"
	"github.com/neurealm/multi-tenant-system/internal/permission"
	"github.com/neurealm/multi-tenant-system/internal/response"
	"github.com/neurealm/multi-tenant-system/internal/role"
	"github.com/neurealm/multi-tenant-system/internal/tenant"
	"github.com/neurealm/multi-tenant-system/internal/user"
)

type Server struct {
	cfg        *config.Config
	pool       *pgxpool.Pool
	router     *chi.Mux
	cache      *cache.PermissionCache
	jwtService *auth.JWTService

	// Handlers
	authHandler       *auth.Handler
	tenantHandler     *tenant.Handler
	userHandler       *user.Handler
	roleHandler       *role.Handler
	permissionHandler *permission.Handler
	auditHandler      *audit.Handler

	// Services
	roleService *role.Service
	userService *user.Service
}

func NewServer(cfg *config.Config, pool *pgxpool.Pool) *Server {
	// 1. Core Services & Cache
	permCache := cache.NewPermissionCache(cfg.CacheTTL, cfg.CacheCleanupPeriod)
	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)

	// 2. Repositories
	auditRepo := audit.NewRepository(pool)
	tenantRepo := tenant.NewRepository(pool)
	roleRepo := role.NewRepository(pool)
	userRepo := user.NewRepository(pool)
	authRepo := auth.NewRepository(pool)
	permRepo := permission.NewRepository(pool)

	// 3. Domain Services
	auditSvc := audit.NewService(auditRepo)
	tenantSvc := tenant.NewService(tenantRepo, pool, auditSvc)
	roleSvc := role.NewService(roleRepo, permCache, auditSvc)
	userSvc := user.NewService(userRepo, permCache, auditSvc)
	permSvc := permission.NewService(permRepo)
	authSvc := auth.NewService(authRepo, userRepo, tenantRepo, roleRepo, jwtSvc, permCache, auditSvc)

	// 4. Handlers
	authH := auth.NewHandler(authSvc, userSvc, tenantRepo, roleSvc)
	tenantH := tenant.NewHandler(tenantSvc)
	userH := user.NewHandler(userSvc, roleSvc)
	roleH := role.NewHandler(roleSvc)
	permH := permission.NewHandler(permSvc)
	auditH := audit.NewHandler(auditSvc)

	s := &Server{
		cfg:               cfg,
		pool:              pool,
		router:            chi.NewRouter(),
		cache:             permCache,
		jwtService:        jwtSvc,
		authHandler:       authH,
		tenantHandler:     tenantH,
		userHandler:       userH,
		roleHandler:       roleH,
		permissionHandler: permH,
		auditHandler:      auditH,
		roleService:       roleSvc,
		userService:       userSvc,
	}

	s.setupRoutes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) Cache() *cache.PermissionCache {
	return s.cache
}

func (s *Server) setupRoutes() {
	r := s.router

	// Global Middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recoverer())

	// CORS Configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-Scope"},
		ExposedHeaders:   []string{"Link", "X-Tenant-Scope"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if s.pool != nil {
			if err := s.pool.Ping(ctx); err != nil {
				response.Error(w, http.StatusServiceUnavailable, "DB_DOWN", "database connection failed", nil)
				return
			}
		}
		response.Success(w, http.StatusOK, "healthy", map[string]interface{}{
			"status":    "UP",
			"timestamp": time.Now().UTC(),
		})
	})

	// API v1 Sub-router
	r.Route(s.cfg.APIBasePath, func(v1 chi.Router) {
		// Public Authentication Routes
		v1.Group(func(pub chi.Router) {
			pub.Post("/auth/login", s.authHandler.Login)
			pub.Post("/auth/refresh", s.authHandler.Refresh)
		})

		// Protected Routes (Token authentication & strict tenant isolation)
		v1.Group(func(prot chi.Router) {
			prot.Use(middleware.Auth(s.jwtService))
			prot.Use(middleware.EnforceTenantIsolation())

			// Auth Session Management
			prot.Get("/auth/me", s.authHandler.Me)
			prot.Post("/auth/switch-tenant", s.authHandler.SwitchTenant)
			prot.Post("/auth/logout", s.authHandler.Logout)

			// Tenants Management
			prot.Route("/tenants", func(tr chi.Router) {
				tr.Post("/", s.tenantHandler.CreateTenant)
				tr.Get("/", s.tenantHandler.ListTenants)
				tr.With(middleware.RequirePermission(s.roleService, domain.PermTenantRead)).Get("/{id}", s.tenantHandler.GetTenant)
				tr.With(middleware.RequirePermission(s.roleService, domain.PermTenantUpdate)).Put("/{id}", s.tenantHandler.UpdateTenant)
			})

			// Users Management (Strictly Tenant-Scoped)
			prot.Route("/users", func(ur chi.Router) {
				ur.With(middleware.RequirePermission(s.roleService, domain.PermUserCreate)).Post("/", s.userHandler.CreateUser)
				ur.With(middleware.RequirePermission(s.roleService, domain.PermUserRead)).Get("/", s.userHandler.ListUsers)
				ur.With(middleware.RequirePermission(s.roleService, domain.PermUserRead)).Get("/{id}", s.userHandler.GetUser)
				ur.With(middleware.RequirePermission(s.roleService, domain.PermUserUpdate)).Put("/{id}", s.userHandler.UpdateUser)
				ur.With(middleware.RequirePermission(s.roleService, domain.PermUserDelete)).Delete("/{id}", s.userHandler.DeleteUser)
				ur.With(middleware.RequirePermission(s.roleService, domain.PermRoleAssign)).Post("/{id}/roles", s.userHandler.AssignRoles)
			})

			// Roles Management (Strictly Tenant-Scoped)
			prot.Route("/roles", func(rr chi.Router) {
				rr.With(middleware.RequirePermission(s.roleService, domain.PermRoleCreate)).Post("/", s.roleHandler.CreateRole)
				rr.With(middleware.RequirePermission(s.roleService, domain.PermRoleRead)).Get("/", s.roleHandler.ListRoles)
				rr.With(middleware.RequirePermission(s.roleService, domain.PermRoleRead)).Get("/{id}", s.roleHandler.GetRole)
				rr.With(middleware.RequirePermission(s.roleService, domain.PermRoleUpdate)).Put("/{id}", s.roleHandler.UpdateRole)
				rr.With(middleware.RequirePermission(s.roleService, domain.PermRoleDelete)).Delete("/{id}", s.roleHandler.DeleteRole)
				rr.With(middleware.RequirePermission(s.roleService, domain.PermRoleUpdate)).Post("/{id}/permissions", s.roleHandler.AssignPermissions)
			})

			// Permissions Catalog
			prot.Route("/permissions", func(pr chi.Router) {
				pr.With(middleware.RequirePermission(s.roleService, domain.PermRoleRead)).Get("/", s.permissionHandler.ListPermissions)
			})

			// Audit Logs (Strictly Tenant-Scoped)
			prot.Route("/audit-logs", func(ar chi.Router) {
				ar.With(middleware.RequirePermission(s.roleService, domain.PermAuditRead)).Get("/", s.auditHandler.ListAuditLogs)
			})
		})
	})
}
