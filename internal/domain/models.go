package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tenant Status constants
const (
	TenantStatusActive    = "active"
	TenantStatusSuspended = "suspended"
)

// User / TenantUser Status constants
const (
	UserStatusActive    = "active"
	UserStatusSuspended = "suspended"
)

// Standard Permission Names (resource:action)
const (
	PermUserCreate = "user:create"
	PermUserRead   = "user:read"
	PermUserUpdate = "user:update"
	PermUserDelete = "user:delete"

	PermRoleCreate = "role:create"
	PermRoleRead   = "role:read"
	PermRoleUpdate = "role:update"
	PermRoleDelete = "role:delete"
	PermRoleAssign = "role:assign"

	PermTenantRead   = "tenant:read"
	PermTenantUpdate = "tenant:update"

	PermAuditRead = "audit:read"
)

// Tenant represents an organization/tenant in the system
type Tenant struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a global user identity
type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	IsPlatformAdmin bool       `json:"is_platform_admin"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	// Tenant-specific fields when fetched in tenant context:
	TenantID        *uuid.UUID `json:"tenant_id,omitempty"`
	TenantStatus    *string    `json:"tenant_status,omitempty"`
	Roles           []Role     `json:"roles,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

// TenantUser represents the association between a User and a Tenant
type TenantUser struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	UserID    uuid.UUID  `json:"user_id"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// Permission represents a fine-grained resource:action grant
type Permission struct {
	ID          uuid.UUID `json:"id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

func (p Permission) String() string {
	return p.Resource + ":" + p.Action
}

// Role represents a tenant-specific role
type Role struct {
	ID          uuid.UUID    `json:"id"`
	TenantID    uuid.UUID    `json:"tenant_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	IsSystem    bool         `json:"is_system"`
	Permissions []Permission `json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// UserRole represents a role assignment to a user in a tenant
type UserRole struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	UserID    uuid.UUID `json:"user_id"`
	RoleID    uuid.UUID `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

// RefreshToken represents a stored refresh token for session tracking & revocation
type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	TokenHash string    `json:"-"`
	UserID    uuid.UUID `json:"user_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Revoked   bool      `json:"revoked"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditLog tracks security and data modification events
type AuditLog struct {
	ID         uuid.UUID              `json:"id"`
	TenantID   *uuid.UUID             `json:"tenant_id,omitempty"`
	ActorID    *uuid.UUID             `json:"actor_id,omitempty"`
	ActorEmail string                 `json:"actor_email"`
	Action     string                 `json:"action"`
	TargetType string                 `json:"target_type"`
	TargetID   string                 `json:"target_id"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
}

// PaginationParams encapsulates standard pagination inputs
type PaginationParams struct {
	Page   int    `json:"page"`
	Limit  int    `json:"limit"`
	Search string `json:"search,omitempty"`
	Status string `json:"status,omitempty"`
}

func (p PaginationParams) Offset() int {
	if p.Page <= 1 {
		return 0
	}
	return (p.Page - 1) * p.Limit
}
