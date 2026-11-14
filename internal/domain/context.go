package domain

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	TenantIDContextKey        contextKey = "tenant_id"
	TenantCodeContextKey      contextKey = "tenant_code"
	UserIDContextKey          contextKey = "user_id"
	UserEmailContextKey       contextKey = "user_email"
	UserRolesContextKey       contextKey = "user_roles"
	UserPermissionsContextKey contextKey = "user_permissions"
	IsPlatformAdminContextKey contextKey = "is_platform_admin"
)

// WithTenantID sets the TenantID in context
func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, TenantIDContextKey, tenantID)
}

// GetTenantID retrieves the TenantID from context
func GetTenantID(ctx context.Context) (uuid.UUID, bool) {
	val := ctx.Value(TenantIDContextKey)
	if val == nil {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

// WithTenantCode sets the TenantCode in context
func WithTenantCode(ctx context.Context, code string) context.Context {
	return context.WithValue(ctx, TenantCodeContextKey, code)
}

// GetTenantCode retrieves TenantCode from context
func GetTenantCode(ctx context.Context) string {
	if val, ok := ctx.Value(TenantCodeContextKey).(string); ok {
		return val
	}
	return ""
}

// WithUserID sets UserID in context
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, UserIDContextKey, userID)
}

// GetUserID retrieves UserID from context
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	val := ctx.Value(UserIDContextKey)
	if val == nil {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

// WithUserEmail sets UserEmail in context
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, UserEmailContextKey, email)
}

// GetUserEmail retrieves UserEmail from context
func GetUserEmail(ctx context.Context) string {
	if val, ok := ctx.Value(UserEmailContextKey).(string); ok {
		return val
	}
	return ""
}

// WithUserRoles sets roles list in context
func WithUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, UserRolesContextKey, roles)
}

// GetUserRoles retrieves roles list from context
func GetUserRoles(ctx context.Context) []string {
	if val, ok := ctx.Value(UserRolesContextKey).([]string); ok {
		return val
	}
	return nil
}

// WithUserPermissions sets permissions list in context
func WithUserPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, UserPermissionsContextKey, permissions)
}

// GetUserPermissions retrieves permissions list from context
func GetUserPermissions(ctx context.Context) []string {
	if val, ok := ctx.Value(UserPermissionsContextKey).([]string); ok {
		return val
	}
	return nil
}

// WithPlatformAdmin sets IsPlatformAdmin in context
func WithPlatformAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, IsPlatformAdminContextKey, isAdmin)
}

// GetPlatformAdmin retrieves IsPlatformAdmin from context
func GetPlatformAdmin(ctx context.Context) bool {
	if val, ok := ctx.Value(IsPlatformAdminContextKey).(bool); ok {
		return val
	}
	return false
}
