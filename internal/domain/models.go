package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	TenantStatusActive    = "active"
	TenantStatusSuspended = "suspended"
)

const (
	UserStatusActive    = "active"
	UserStatusSuspended = "suspended"
)

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

type Tenant struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type User struct {
	ID              uuid.UUID `json:"id"`
	Email           string    `json:"email"`
	PasswordHash    string    `json:"-"`
	FirstName       string    `json:"first_name"`
	LastName        string    `json:"last_name"`
	IsPlatformAdmin bool      `json:"is_platform_admin"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	TenantID     *uuid.UUID `json:"tenant_id,omitempty"`
	TenantStatus *string    `json:"tenant_status,omitempty"`
	Roles        []Role     `json:"roles,omitempty"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

type TenantUser struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	UserID    uuid.UUID  `json:"user_id"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

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

type UserRole struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	UserID    uuid.UUID `json:"user_id"`
	RoleID    uuid.UUID `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	TokenHash string    `json:"-"`
	UserID    uuid.UUID `json:"user_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Revoked   bool      `json:"revoked"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

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
