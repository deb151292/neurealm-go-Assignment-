package server

import (
	"context"
	"net/http"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/docs"
	"github.com/deb151292/neurealm-go-Assignment-/internal/audit"
	"github.com/deb151292/neurealm-go-Assignment-/internal/auth"
	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/deb151292/neurealm-go-Assignment-/internal/config"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/middleware"
	"github.com/deb151292/neurealm-go-Assignment-/internal/permission"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/deb151292/neurealm-go-Assignment-/internal/role"
	"github.com/deb151292/neurealm-go-Assignment-/internal/tenant"
	"github.com/deb151292/neurealm-go-Assignment-/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg        *config.Config
	pool       *pgxpool.Pool
	engine     *gin.Engine
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
	if cfg.Env == "production" || cfg.Env == "test" {
		gin.SetMode(gin.ReleaseMode)
	}

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

	engine := gin.New()

	s := &Server{
		cfg:               cfg,
		pool:              pool,
		engine:            engine,
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

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) Router() http.Handler {
	return s.engine
}

func (s *Server) Cache() *cache.PermissionCache {
	return s.cache
}

func (s *Server) setupRoutes() {
	r := s.engine

	// Global Middlewares
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recoverer())

	// Custom CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Tenant-Scope")
		c.Header("Access-Control-Expose-Headers", "Link, X-Tenant-Scope")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// Mount Swagger UI & OpenAPI Specification routes
	docs.RegisterSwaggerRoutes(r)

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if s.pool != nil {
			if err := s.pool.Ping(ctx); err != nil {
				response.Error(c, http.StatusServiceUnavailable, "DB_DOWN", "database connection failed", nil)
				return
			}
		}
		response.Success(c, http.StatusOK, "healthy", map[string]interface{}{
			"status":    "UP",
			"timestamp": time.Now().UTC(),
		})
	})

	// API v1 Sub-router Group
	v1 := r.Group(s.cfg.APIBasePath)
	{
		// Public Authentication Routes
		v1.POST("/auth/login", s.authHandler.Login)
		v1.POST("/auth/refresh", s.authHandler.Refresh)

		// Protected Routes (Token authentication & strict tenant isolation)
		prot := v1.Group("")
		prot.Use(middleware.Auth(s.jwtService))
		prot.Use(middleware.EnforceTenantIsolation())
		{
			// Auth Session Management
			prot.GET("/auth/me", s.authHandler.Me)
			prot.POST("/auth/switch-tenant", s.authHandler.SwitchTenant)
			prot.POST("/auth/logout", s.authHandler.Logout)

			// Tenants Management
			tenants := prot.Group("/tenants")
			{
				tenants.POST("", s.tenantHandler.CreateTenant)
				tenants.GET("", s.tenantHandler.ListTenants)
				tenants.GET("/:id", middleware.RequirePermission(s.roleService, domain.PermTenantRead), s.tenantHandler.GetTenant)
				tenants.PUT("/:id", middleware.RequirePermission(s.roleService, domain.PermTenantUpdate), s.tenantHandler.UpdateTenant)
			}

			// Users Management (Strictly Tenant-Scoped)
			users := prot.Group("/users")
			{
				users.POST("", middleware.RequirePermission(s.roleService, domain.PermUserCreate), s.userHandler.CreateUser)
				users.GET("", middleware.RequirePermission(s.roleService, domain.PermUserRead), s.userHandler.ListUsers)
				users.GET("/:id", middleware.RequirePermission(s.roleService, domain.PermUserRead), s.userHandler.GetUser)
				users.PUT("/:id", middleware.RequirePermission(s.roleService, domain.PermUserUpdate), s.userHandler.UpdateUser)
				users.DELETE("/:id", middleware.RequirePermission(s.roleService, domain.PermUserDelete), s.userHandler.DeleteUser)
				users.POST("/:id/roles", middleware.RequirePermission(s.roleService, domain.PermRoleAssign), s.userHandler.AssignRoles)
			}

			// Roles Management (Strictly Tenant-Scoped)
			roles := prot.Group("/roles")
			{
				roles.POST("", middleware.RequirePermission(s.roleService, domain.PermRoleCreate), s.roleHandler.CreateRole)
				roles.GET("", middleware.RequirePermission(s.roleService, domain.PermRoleRead), s.roleHandler.ListRoles)
				roles.GET("/:id", middleware.RequirePermission(s.roleService, domain.PermRoleRead), s.roleHandler.GetRole)
				roles.PUT("/:id", middleware.RequirePermission(s.roleService, domain.PermRoleUpdate), s.roleHandler.UpdateRole)
				roles.DELETE("/:id", middleware.RequirePermission(s.roleService, domain.PermRoleDelete), s.roleHandler.DeleteRole)
				roles.POST("/:id/permissions", middleware.RequirePermission(s.roleService, domain.PermRoleUpdate), s.roleHandler.AssignPermissions)
			}

			// Permissions Catalog
			permissions := prot.Group("/permissions")
			{
				permissions.GET("", middleware.RequirePermission(s.roleService, domain.PermRoleRead), s.permissionHandler.ListPermissions)
			}

			// Audit Logs (Strictly Tenant-Scoped)
			auditLogs := prot.Group("/audit-logs")
			{
				auditLogs.GET("", middleware.RequirePermission(s.roleService, domain.PermAuditRead), s.auditHandler.ListAuditLogs)
			}
		}
	}
}
