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
	"github.com/deb151292/neurealm-go-Assignment-/internal/role"
	"github.com/google/uuid"
)

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	jwtSvc := auth.NewJWTService("my-super-secret-key-32-chars-long!", 15*time.Minute, 24*time.Hour)
	handler := middleware.Auth(jwtSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

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
	handler := middleware.Auth(jwtSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer invalid-garbage-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

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

	handler := middleware.Auth(jwtSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID, _ = domain.GetTenantID(r.Context())
		capturedUserID, _ = domain.GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

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
	// Initialize cache without DB connection
	permCache := cache.NewPermissionCache(5*time.Minute, 10*time.Minute)
	defer permCache.Stop()

	tenantID := uuid.New()
	userID := uuid.New()

	// Cache user with only "user:read" permission
	permCache.Set(tenantID, userID, []string{"user:read"})

	roleSvc := role.NewService(nil, permCache, nil)
	handler := middleware.RequirePermission(roleSvc, "user:create")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/users", nil)
	ctx := domain.WithTenantID(req.Context(), tenantID)
	ctx = domain.WithUserID(ctx, userID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req.WithContext(ctx))

	// User has "user:read" but requested action requires "user:create" -> 403 Forbidden!
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

	// Cache user with "user:create" permission
	permCache.Set(tenantID, userID, []string{"user:create", "user:read"})

	roleSvc := role.NewService(nil, permCache, nil)
	handler := middleware.RequirePermission(roleSvc, "user:create")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))

	req := httptest.NewRequest("POST", "/api/v1/users", nil)
	ctx := domain.WithTenantID(req.Context(), tenantID)
	ctx = domain.WithUserID(ctx, userID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}
