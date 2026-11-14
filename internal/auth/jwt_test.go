package auth_test

import (
	"testing"
	"time"

	"github.com/deb151292/neurealm-go-Assignment-/internal/auth"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/google/uuid"
)

func TestJWTService_GenerateAndValidateAccessToken(t *testing.T) {
	svc := auth.NewJWTService("my-super-secret-key-32-chars-long!", 15*time.Minute, 24*time.Hour)

	user := &domain.User{
		ID:              uuid.New(),
		Email:           "tester@example.com",
		FirstName:       "Test",
		LastName:        "User",
		IsPlatformAdmin: false,
	}

	tenant := &domain.Tenant{
		ID:     uuid.New(),
		Name:   "Test Org",
		Code:   "test-org",
		Status: domain.TenantStatusActive,
	}

	roles := []string{"Admin", "Billing"}
	permissions := []string{"user:create", "user:read", "role:assign"}

	tokenStr, err := svc.GenerateAccessToken(user, tenant, roles, permissions)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	claims, err := svc.ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("failed to validate valid access token: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("expected userID %s, got %s", user.ID, claims.UserID)
	}
	if claims.TenantID != tenant.ID {
		t.Errorf("expected tenantID %s, got %s", tenant.ID, claims.TenantID)
	}
	if claims.TenantCode != tenant.Code {
		t.Errorf("expected tenantCode %s, got %s", tenant.Code, claims.TenantCode)
	}
	if len(claims.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(claims.Roles))
	}
	if len(claims.Permissions) != 3 {
		t.Errorf("expected 3 permissions, got %d", len(claims.Permissions))
	}
}

func TestJWTService_TamperedSecret(t *testing.T) {
	svc1 := auth.NewJWTService("secret-key-number-one-32-bytes!!", 15*time.Minute, 24*time.Hour)
	svc2 := auth.NewJWTService("secret-key-number-two-different!!", 15*time.Minute, 24*time.Hour)

	user := &domain.User{ID: uuid.New(), Email: "test@example.com"}
	tenant := &domain.Tenant{ID: uuid.New(), Code: "test"}

	token, _ := svc1.GenerateAccessToken(user, tenant, nil, nil)

	_, err := svc2.ValidateAccessToken(token)
	if err == nil {
		t.Fatalf("expected validation error with mismatched secret")
	}
}

func TestJWTService_ExpiredToken(t *testing.T) {

	svc := auth.NewJWTService("my-super-secret-key-32-chars-long!", -1*time.Minute, 24*time.Hour)

	user := &domain.User{ID: uuid.New(), Email: "test@example.com"}
	tenant := &domain.Tenant{ID: uuid.New(), Code: "test"}

	token, _ := svc.GenerateAccessToken(user, tenant, nil, nil)

	_, err := svc.ValidateAccessToken(token)
	if err == nil {
		t.Fatalf("expected error for expired token")
	}
}

func TestJWTService_RefreshTokenHash(t *testing.T) {
	svc := auth.NewJWTService("secret", 15*time.Minute, 24*time.Hour)

	raw, hash, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	if len(raw) == 0 || len(hash) == 0 {
		t.Fatalf("empty token or hash generated")
	}

	reHash := svc.HashRefreshToken(raw)
	if reHash != hash {
		t.Fatalf("recalculated hash does not match original hash")
	}
}
