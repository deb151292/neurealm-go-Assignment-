package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/neurealm/multi-tenant-system/internal/audit"
	"github.com/neurealm/multi-tenant-system/internal/auth"
	"github.com/neurealm/multi-tenant-system/internal/cache"
	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/middleware"
	"github.com/neurealm/multi-tenant-system/internal/permission"
	"github.com/neurealm/multi-tenant-system/internal/role"
	"github.com/neurealm/multi-tenant-system/internal/tenant"
	"github.com/neurealm/multi-tenant-system/internal/user"
	"golang.org/x/crypto/bcrypt"
)

type TestEnv struct {
	Store      *MockStore
	Router     http.Handler
	JWTService *auth.JWTService
	Cache      *cache.PermissionCache

	TenantA *domain.Tenant
	TenantB *domain.Tenant

	UserAInTenantA *domain.User
	UserBInTenantB *domain.User
	MultiUserAlice *domain.User

	TokenA string
	TokenB string
}

func setupTestEnv(t *testing.T) *TestEnv {
	store := NewMockStore()
	permCache := cache.NewPermissionCache(5*time.Minute, 10*time.Minute)
	jwtSvc := auth.NewJWTService("integration-test-secret-32-bytes-long!", 15*time.Minute, 24*time.Hour)

	auditSvc := audit.NewService(store.AuditRepo())
	tenantSvc := tenant.NewService(store.TenantRepo(), nil, auditSvc)
	roleSvc := role.NewService(store.RoleRepo(), permCache, auditSvc)
	userSvc := user.NewService(store.UserRepo(), permCache, auditSvc)
	permSvc := permission.NewService(store.PermRepo())
	authSvc := auth.NewService(store.AuthRepo(), store.UserRepo(), store.TenantRepo(), store.RoleRepo(), jwtSvc, permCache, auditSvc)

	tenantH := tenant.NewHandler(tenantSvc)
	userH := user.NewHandler(userSvc, roleSvc)
	roleH := role.NewHandler(roleSvc)
	permH := permission.NewHandler(permSvc)
	auditH := audit.NewHandler(auditSvc)
	authH := auth.NewHandler(authSvc, userSvc, store.TenantRepo(), roleSvc)

	r := chi.NewRouter()
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Post("/auth/login", authH.Login)
		v1.Post("/auth/refresh", authH.Refresh)

		v1.Group(func(prot chi.Router) {
			prot.Use(middleware.Auth(jwtSvc))
			prot.Use(middleware.EnforceTenantIsolation())

			prot.Get("/auth/me", authH.Me)
			prot.Post("/auth/switch-tenant", authH.SwitchTenant)
			prot.Post("/auth/logout", authH.Logout)

			prot.Route("/tenants", func(tr chi.Router) {
				tr.Post("/", tenantH.CreateTenant)
				tr.Get("/", tenantH.ListTenants)
				tr.With(middleware.RequirePermission(roleSvc, domain.PermTenantRead)).Get("/{id}", tenantH.GetTenant)
				tr.With(middleware.RequirePermission(roleSvc, domain.PermTenantUpdate)).Put("/{id}", tenantH.UpdateTenant)
			})

			prot.Route("/users", func(ur chi.Router) {
				ur.With(middleware.RequirePermission(roleSvc, domain.PermUserCreate)).Post("/", userH.CreateUser)
				ur.With(middleware.RequirePermission(roleSvc, domain.PermUserRead)).Get("/", userH.ListUsers)
				ur.With(middleware.RequirePermission(roleSvc, domain.PermUserRead)).Get("/{id}", userH.GetUser)
				ur.With(middleware.RequirePermission(roleSvc, domain.PermUserUpdate)).Put("/{id}", userH.UpdateUser)
				ur.With(middleware.RequirePermission(roleSvc, domain.PermUserDelete)).Delete("/{id}", userH.DeleteUser)
				ur.With(middleware.RequirePermission(roleSvc, domain.PermRoleAssign)).Post("/{id}/roles", userH.AssignRoles)
			})

			prot.Route("/roles", func(rr chi.Router) {
				rr.With(middleware.RequirePermission(roleSvc, domain.PermRoleCreate)).Post("/", roleH.CreateRole)
				rr.With(middleware.RequirePermission(roleSvc, domain.PermRoleRead)).Get("/", roleH.ListRoles)
				rr.With(middleware.RequirePermission(roleSvc, domain.PermRoleRead)).Get("/{id}", roleH.GetRole)
				rr.With(middleware.RequirePermission(roleSvc, domain.PermRoleUpdate)).Put("/{id}", roleH.UpdateRole)
				rr.With(middleware.RequirePermission(roleSvc, domain.PermRoleDelete)).Delete("/{id}", roleH.DeleteRole)
				rr.With(middleware.RequirePermission(roleSvc, domain.PermRoleUpdate)).Post("/{id}/permissions", roleH.AssignPermissions)
			})

			prot.Route("/permissions", func(pr chi.Router) {
				pr.With(middleware.RequirePermission(roleSvc, domain.PermRoleRead)).Get("/", permH.ListPermissions)
			})

			prot.Route("/audit-logs", func(ar chi.Router) {
				ar.With(middleware.RequirePermission(roleSvc, domain.PermAuditRead)).Get("/", auditH.ListAuditLogs)
			})
		})
	})

	ctx := context.Background()

	// 1. Create Tenant A & Tenant B
	tA := &domain.Tenant{ID: uuid.New(), Name: "Acme Corp", Code: "acme-corp", Status: domain.TenantStatusActive}
	tB := &domain.Tenant{ID: uuid.New(), Name: "Globex Corporation", Code: "globex", Status: domain.TenantStatusActive}
	_ = store.Create(ctx, tA)
	_ = store.Create(ctx, tB)

	// 2. Create Roles in Tenant A & Tenant B
	allPerms, _ := store.ListAllPermissions(ctx)
	allPermIDs := make([]uuid.UUID, len(allPerms))
	allPermStrings := make([]string, len(allPerms))
	for i, p := range allPerms {
		allPermIDs[i] = p.ID
		allPermStrings[i] = p.String()
	}

	roleAdminA := &domain.Role{ID: uuid.New(), TenantID: tA.ID, Name: "Admin", IsSystem: true}
	_ = store.CreateRole(ctx, roleAdminA)
	_ = store.AssignPermissions(ctx, tA.ID, roleAdminA.ID, allPermIDs)

	roleAdminB := &domain.Role{ID: uuid.New(), TenantID: tB.ID, Name: "Admin", IsSystem: true}
	_ = store.CreateRole(ctx, roleAdminB)
	_ = store.AssignPermissions(ctx, tB.ID, roleAdminB.ID, allPermIDs)

	// 3. Create Users
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	uA := &domain.User{Email: "user.a@acme.com", PasswordHash: string(hash), FirstName: "User", LastName: "Acme"}
	_ = store.CreateUserInTenant(ctx, tA.ID, uA, []uuid.UUID{roleAdminA.ID})

	uB := &domain.User{Email: "user.b@globex.com", PasswordHash: string(hash), FirstName: "User", LastName: "Globex"}
	_ = store.CreateUserInTenant(ctx, tB.ID, uB, []uuid.UUID{roleAdminB.ID})

	// Multi-tenant user Alice
	uAlice := &domain.User{Email: "alice@multi.com", PasswordHash: string(hash), FirstName: "Alice", LastName: "Multi"}
	_ = store.CreateUserInTenant(ctx, tA.ID, uAlice, []uuid.UUID{roleAdminA.ID})
	_ = store.CreateUserInTenant(ctx, tB.ID, uAlice, []uuid.UUID{roleAdminB.ID})

	// Generate Scoped Tokens
	tokenA, _ := jwtSvc.GenerateAccessToken(uA, tA, []string{"Admin"}, allPermStrings)
	tokenB, _ := jwtSvc.GenerateAccessToken(uB, tB, []string{"Admin"}, allPermStrings)

	return &TestEnv{
		Store:          store,
		Router:         r,
		JWTService:     jwtSvc,
		Cache:          permCache,
		TenantA:        tA,
		TenantB:        tB,
		UserAInTenantA: uA,
		UserBInTenantB: uB,
		MultiUserAlice: uAlice,
		TokenA:         tokenA,
		TokenB:         tokenB,
	}
}

// ----------------- CROSS-TENANT ISOLATION TESTS -----------------

func TestCrossTenant_GetUser_Returns404(t *testing.T) {
	env := setupTestEnv(t)

	// Tenant B attempts to read Tenant A's user UserAInTenantA
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/users/%s", env.UserAInTenantA.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.TokenB) // Token from Tenant B!
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	// Cross-tenant access MUST return 404 Not Found to prevent tenant data enumeration
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for cross-tenant user lookup, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res["success"] != false {
		t.Errorf("expected success=false")
	}
}

func TestCrossTenant_UpdateUser_Returns404(t *testing.T) {
	env := setupTestEnv(t)

	payload := `{"first_name": "Hacked", "last_name": "User"}`
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/users/%s", env.UserAInTenantA.ID), bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer "+env.TokenB) // Tenant B token
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for cross-tenant user update, got %d", rec.Code)
	}
}

func TestCrossTenant_DeleteUser_Returns404(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/users/%s", env.UserAInTenantA.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.TokenB) // Tenant B token
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for cross-tenant user deletion, got %d", rec.Code)
	}
}

func TestCrossTenant_GetRole_Returns404(t *testing.T) {
	env := setupTestEnv(t)

	// Create a custom role in Tenant A
	roleA := &domain.Role{ID: uuid.New(), TenantID: env.TenantA.ID, Name: "AcmeCustomRole"}
	_ = env.Store.CreateRole(context.Background(), roleA)

	// Tenant B attempts to read Role A
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/roles/%s", roleA.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.TokenB)
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for cross-tenant role read, got %d", rec.Code)
	}
}

func TestCrossTenant_UpdateRole_Returns404(t *testing.T) {
	env := setupTestEnv(t)

	roleA := &domain.Role{ID: uuid.New(), TenantID: env.TenantA.ID, Name: "AcmeRoleToUpdate"}
	_ = env.Store.CreateRole(context.Background(), roleA)

	payload := `{"name": "TamperedName", "description": "Tampered"}`
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/roles/%s", roleA.ID), bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer "+env.TokenB)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for cross-tenant role update, got %d", rec.Code)
	}
}

func TestCrossTenant_DeleteRole_Returns404(t *testing.T) {
	env := setupTestEnv(t)

	roleA := &domain.Role{ID: uuid.New(), TenantID: env.TenantA.ID, Name: "AcmeRoleToDelete", IsSystem: false}
	_ = env.Store.CreateRole(context.Background(), roleA)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/roles/%s", roleA.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.TokenB)
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for cross-tenant role delete, got %d", rec.Code)
	}
}

func TestCrossTenant_GetTenantDirect_Returns404(t *testing.T) {
	env := setupTestEnv(t)

	// Tenant B token querying Tenant A's ID
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/tenants/%s", env.TenantA.ID), nil)
	req.Header.Set("Authorization", "Bearer "+env.TokenB)
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 when querying another tenant's profile, got %d", rec.Code)
	}
}

func TestCrossTenant_AuditLogsIsolation(t *testing.T) {
	env := setupTestEnv(t)

	// Log event for Tenant A and Tenant B
	_ = env.Store.CreateAudit(context.Background(), &domain.AuditLog{
		TenantID:   &env.TenantA.ID,
		Action:     "user:secret_acme_action",
		TargetType: "user",
		TargetID:   env.UserAInTenantA.ID.String(),
	})

	_ = env.Store.CreateAudit(context.Background(), &domain.AuditLog{
		TenantID:   &env.TenantB.ID,
		Action:     "user:globex_action",
		TargetType: "user",
		TargetID:   env.UserBInTenantB.ID.String(),
	})

	// Tenant B requests audit logs
	req := httptest.NewRequest("GET", "/api/v1/audit-logs", nil)
	req.Header.Set("Authorization", "Bearer "+env.TokenB)
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for audit logs, got %d", rec.Code)
	}

	bodyStr := rec.Body.String()
	if bytes.Contains([]byte(bodyStr), []byte("user:secret_acme_action")) {
		t.Fatalf("DATA LEAK! Tenant B received Tenant A's audit log entry!")
	}
	if !bytes.Contains([]byte(bodyStr), []byte("user:globex_action")) {
		t.Fatalf("Tenant B should receive its own audit log entry")
	}
}

// ----------------- AUTHORIZATION 401 VS 403 TESTS -----------------

func TestAuth_401Unauthorized_NoToken(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for missing token, got %d", rec.Code)
	}
}

func TestAuth_403Forbidden_MissingPermission(t *testing.T) {
	env := setupTestEnv(t)

	// Create role in Tenant A with ONLY user:read permission
	readOnlyRole := &domain.Role{ID: uuid.New(), TenantID: env.TenantA.ID, Name: "Viewer"}
	_ = env.Store.CreateRole(context.Background(), readOnlyRole)

	// Find user:read permission ID
	var readPermID uuid.UUID
	for id, p := range env.Store.Permissions {
		if p.Resource == "user" && p.Action == "read" {
			readPermID = id
			break
		}
	}
	_ = env.Store.AssignPermissions(context.Background(), env.TenantA.ID, readOnlyRole.ID, []uuid.UUID{readPermID})

	// Create user with Viewer role only
	viewerUser := &domain.User{Email: "viewer@acme.com", FirstName: "View", LastName: "Only"}
	_ = env.Store.CreateUserInTenant(context.Background(), env.TenantA.ID, viewerUser, []uuid.UUID{readOnlyRole.ID})

	viewerToken, _ := env.JWTService.GenerateAccessToken(viewerUser, env.TenantA, []string{"Viewer"}, []string{"user:read"})

	// Attempt to create user (requires "user:create")
	payload := `{"email": "newbie@acme.com", "password": "Password123!", "first_name": "New", "last_name": "Bie"}`
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	env.Router.ServeHTTP(rec, req)

	// Must return 403 Forbidden!
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when user lacks permission, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ----------------- SOFT DELETE & MEMBERSHIP TESTS -----------------

func TestUser_SoftDelete_ExcludesFromQuery(t *testing.T) {
	env := setupTestEnv(t)

	// 1. Delete user in Tenant A
	reqDel := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/users/%s", env.UserAInTenantA.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+env.TokenA)
	recDel := httptest.NewRecorder()
	env.Router.ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for soft delete, got %d", recDel.Code)
	}

	// 2. Subsequent GET on soft-deleted user returns 404
	reqGet := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/users/%s", env.UserAInTenantA.ID), nil)
	reqGet.Header.Set("Authorization", "Bearer "+env.TokenA)
	recGet := httptest.NewRecorder()
	env.Router.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for soft deleted user, got %d", recGet.Code)
	}
}

func TestMultiTenant_LoginAndSwitchTenant(t *testing.T) {
	env := setupTestEnv(t)

	// 1. Alice logs in specifying "acme-corp"
	loginReq := auth.LoginRequest{
		Email:      "alice@multi.com",
		Password:   "Password123!",
		TenantCode: "acme-corp",
	}
	body, _ := json.Marshal(loginReq)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 login, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	data := res["data"].(map[string]interface{})
	tokenA := data["access_token"].(string)

	// 2. Alice switches tenant to "globex"
	switchReq := auth.SwitchTenantRequest{TenantCode: "globex"}
	switchBody, _ := json.Marshal(switchReq)
	reqSwitch := httptest.NewRequest("POST", "/api/v1/auth/switch-tenant", bytes.NewReader(switchBody))
	reqSwitch.Header.Set("Authorization", "Bearer "+tokenA)
	reqSwitch.Header.Set("Content-Type", "application/json")
	recSwitch := httptest.NewRecorder()
	env.Router.ServeHTTP(recSwitch, reqSwitch)

	if recSwitch.Code != http.StatusOK {
		t.Fatalf("expected 200 switch-tenant, got %d: %s", recSwitch.Code, recSwitch.Body.String())
	}

	var switchRes map[string]interface{}
	_ = json.Unmarshal(recSwitch.Body.Bytes(), &switchRes)
	switchData := switchRes["data"].(map[string]interface{})
	tokenB := switchData["access_token"].(string)

	claimsB, err := env.JWTService.ValidateAccessToken(tokenB)
	if err != nil {
		t.Fatalf("failed to parse switched token: %v", err)
	}
	if claimsB.TenantCode != "globex" {
		t.Fatalf("expected new token scoped to 'globex', got %s", claimsB.TenantCode)
	}
}
