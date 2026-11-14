package validator_test

import (
	"testing"

	"github.com/neurealm/multi-tenant-system/internal/validator"
)

func TestValidator_Email(t *testing.T) {
	v := validator.New()
	v.Email("valid@example.com", "email")
	if !v.Valid() {
		t.Fatalf("expected valid email, got errors: %v", v.Errors)
	}

	v2 := validator.New()
	v2.Email("invalid-email-address", "email")
	if v2.Valid() {
		t.Fatalf("expected validation error for invalid email")
	}
}

func TestValidator_TenantCode(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"acme-corp", true},
		{"globex", true},
		{"org-123", true},
		{"Abc", false},     // uppercase
		{"a", false},       // too short
		{"org_name", false}, // underscores not permitted in slug
		{"org--name", false}, // double hyphen
		{"-org", false},
	}

	for _, tt := range tests {
		v := validator.New()
		v.TenantCode(tt.code, "code")
		if v.Valid() != tt.valid {
			t.Errorf("code %q expected valid=%v, got valid=%v", tt.code, tt.valid, v.Valid())
		}
	}
}

func TestValidator_In(t *testing.T) {
	v := validator.New()
	v.In("active", "status", "active", "suspended")
	if !v.Valid() {
		t.Errorf("expected valid status")
	}

	v2 := validator.New()
	v2.In("deleted", "status", "active", "suspended")
	if v2.Valid() {
		t.Errorf("expected validation failure for 'deleted'")
	}
}
