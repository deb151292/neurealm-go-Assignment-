package server

import (
	"context"
	"net/http"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/docs"
	"github.com/deb151292/neurealm-go-Assignment-/internal/auth"
	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/deb151292/neurealm-go-Assignment-/internal/config"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/handler"
	"github.com/deb151292/neurealm-go-Assignment-/internal/middleware"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg        *config.Config
	pool       *pgxpool.Pool
	engine     *gin.Engine
	cache      *cache.PermissionCache
	jwtService *auth.JWTService
	handlers   *handler.Handlers
}

func NewServer(cfg *config.Config, pool *pgxpool.Pool) *Server {
	if cfg.Env == "production" || cfg.Env == "test" {
		gin.SetMode(gin.ReleaseMode)
	}

	permCache := cache.NewPermissionCache(cfg.CacheTTL, cfg.CacheCleanupPeriod)
	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)

	handlers := handler.NewHandlers(pool, jwtSvc, permCache)

	engine := gin.New()

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		engine:     engine,
		cache:      permCache,
		jwtService: jwtSvc,
		handlers:   handlers,
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

func (s *Server) Handlers() *handler.Handlers {
	return s.handlers
}

func (s *Server) setupRoutes() {
	r := s.engine
	h := s.handlers

	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recoverer())

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

	docs.RegisterSwaggerRoutes(r)

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

	v1 := r.Group(s.cfg.APIBasePath)
	{

		v1.POST("/auth/login", h.Auth.Login)
		v1.POST("/auth/refresh", h.Auth.Refresh)

		prot := v1.Group("")
		prot.Use(middleware.Auth(s.jwtService))
		prot.Use(middleware.EnforceTenantIsolation())
		{

			prot.GET("/auth/me", h.Auth.Me)
			prot.POST("/auth/switch-tenant", h.Auth.SwitchTenant)
			prot.POST("/auth/logout", h.Auth.Logout)

			tenants := prot.Group("/tenants")
			{
				tenants.POST("", h.Tenant.CreateTenant)
				tenants.GET("", h.Tenant.ListTenants)
				tenants.GET("/:id", middleware.RequirePermission(h.RoleService, domain.PermTenantRead), h.Tenant.GetTenant)
				tenants.PUT("/:id", middleware.RequirePermission(h.RoleService, domain.PermTenantUpdate), h.Tenant.UpdateTenant)
			}

			users := prot.Group("/users")
			{
				users.POST("", middleware.RequirePermission(h.RoleService, domain.PermUserCreate), h.User.CreateUser)
				users.GET("", middleware.RequirePermission(h.RoleService, domain.PermUserRead), h.User.ListUsers)
				users.GET("/:id", middleware.RequirePermission(h.RoleService, domain.PermUserRead), h.User.GetUser)
				users.PUT("/:id", middleware.RequirePermission(h.RoleService, domain.PermUserUpdate), h.User.UpdateUser)
				users.DELETE("/:id", middleware.RequirePermission(h.RoleService, domain.PermUserDelete), h.User.DeleteUser)
				users.POST("/:id/roles", middleware.RequirePermission(h.RoleService, domain.PermRoleAssign), h.User.AssignRoles)
			}

			roles := prot.Group("/roles")
			{
				roles.POST("", middleware.RequirePermission(h.RoleService, domain.PermRoleCreate), h.Role.CreateRole)
				roles.GET("", middleware.RequirePermission(h.RoleService, domain.PermRoleRead), h.Role.ListRoles)
				roles.GET("/:id", middleware.RequirePermission(h.RoleService, domain.PermRoleRead), h.Role.GetRole)
				roles.PUT("/:id", middleware.RequirePermission(h.RoleService, domain.PermRoleUpdate), h.Role.UpdateRole)
				roles.DELETE("/:id", middleware.RequirePermission(h.RoleService, domain.PermRoleDelete), h.Role.DeleteRole)
				roles.POST("/:id/permissions", middleware.RequirePermission(h.RoleService, domain.PermRoleUpdate), h.Role.AssignPermissions)
			}

			permissions := prot.Group("/permissions")
			{
				permissions.GET("", middleware.RequirePermission(h.RoleService, domain.PermRoleRead), h.Permission.ListPermissions)
			}

			auditLogs := prot.Group("/audit-logs")
			{
				auditLogs.GET("", middleware.RequirePermission(h.RoleService, domain.PermAuditRead), h.Audit.ListAuditLogs)
			}
		}
	}
}
