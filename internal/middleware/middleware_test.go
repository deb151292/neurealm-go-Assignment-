package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/auth"
	"github.com/deb151292/neurealm-go-Assignment-/internal/cache"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/middleware"
	"github.com/deb151292/neurealm-go-Assignment-/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	jwtSvc := auth.NewJWTService("my-super-secret-key-32-chars-long!", 15*time.Minute, 24*time.Hour)
	r := gin.New()
	r.Use(middleware.Auth(jwtSvc))
	r.GET("/api/v1/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res["success"] != false {
		t.Errorf("expected success=false")
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	jwtSvc := auth.NewJWTService("my-super-secret-key-32-chars-long!", 15*time.Minute, 24*time.Hour)
	r := gin.New()
	r.Use(middleware.Auth(jwtSvc))
	r.GET("/api/v1/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer invalid-garbage-token")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	jwtSvc := auth.NewJWTService("my-super-secret-key-32-chars-long!", 15*time.Minute, 24*time.Hour)
	user := &domain.User{ID: uuid.New(), Email: "test@example.com"}
	tenant := &domain.Tenant{ID: uuid.New(), Code: "org1"}
	tokenStr, _ := jwtSvc.GenerateAccessToken(user, tenant, []string{"Admin"}, []string{"user:read"})

	var capturedTenantID uuid.UUID
	var capturedUserID uuid.UUID

	r := gin.New()
	r.Use(middleware.Auth(jwtSvc))
	r.GET("/api/v1/users", func(c *gin.Context) {
		capturedTenantID, _ = domain.GetTenantID(c.Request.Context())
		capturedUserID, _ = domain.GetUserID(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if capturedTenantID != tenant.ID {
		t.Errorf("expected tenantID %s, got %s", tenant.ID, capturedTenantID)
	}
	if capturedUserID != user.ID {
		t.Errorf("expected userID %s, got %s", user.ID, capturedUserID)
	}
}

func TestRBACMiddleware_403Forbidden(t *testing.T) {
	permCache := cache.NewPermissionCache(5*time.Minute, 10*time.Minute)
	defer permCache.Stop()

	tenantID := uuid.New()
	userID := uuid.New()

	permCache.Set(tenantID, userID, []string{"user:read"})

	roleSvc := service.NewRoleService(nil, permCache, nil)
	r := gin.New()
	r.POST("/api/v1/users", middleware.RequirePermission(roleSvc, "user:create"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/api/v1/users", nil)
	ctx := domain.WithTenantID(req.Context(), tenantID)
	ctx = domain.WithUserID(ctx, userID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	errObj := res["error"].(map[string]interface{})
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %v", errObj["code"])
	}
}

func TestRBACMiddleware_200Success(t *testing.T) {
	permCache := cache.NewPermissionCache(5*time.Minute, 10*time.Minute)
	defer permCache.Stop()

	tenantID := uuid.New()
	userID := uuid.New()

	permCache.Set(tenantID, userID, []string{"user:create", "user:read"})

	roleSvc := service.NewRoleService(nil, permCache, nil)
	r := gin.New()
	r.POST("/api/v1/users", middleware.RequirePermission(roleSvc, "user:create"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	req := httptest.NewRequest("POST", "/api/v1/users", nil)
	ctx := domain.WithTenantID(req.Context(), tenantID)
	ctx = domain.WithUserID(ctx, userID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}
