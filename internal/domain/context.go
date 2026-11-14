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

func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, TenantIDContextKey, tenantID)
}

func GetTenantID(ctx context.Context) (uuid.UUID, bool) {
	val := ctx.Value(TenantIDContextKey)
	if val == nil {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

func WithTenantCode(ctx context.Context, code string) context.Context {
	return context.WithValue(ctx, TenantCodeContextKey, code)
}

func GetTenantCode(ctx context.Context) string {
	if val, ok := ctx.Value(TenantCodeContextKey).(string); ok {
		return val
	}
	return ""
}

func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, UserIDContextKey, userID)
}

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	val := ctx.Value(UserIDContextKey)
	if val == nil {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, UserEmailContextKey, email)
}

func GetUserEmail(ctx context.Context) string {
	if val, ok := ctx.Value(UserEmailContextKey).(string); ok {
		return val
	}
	return ""
}

func WithUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, UserRolesContextKey, roles)
}

func GetUserRoles(ctx context.Context) []string {
	if val, ok := ctx.Value(UserRolesContextKey).([]string); ok {
		return val
	}
	return nil
}

func WithUserPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, UserPermissionsContextKey, permissions)
}

func GetUserPermissions(ctx context.Context) []string {
	if val, ok := ctx.Value(UserPermissionsContextKey).([]string); ok {
		return val
	}
	return nil
}

func WithPlatformAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, IsPlatformAdminContextKey, isAdmin)
}

func GetPlatformAdmin(ctx context.Context) bool {
	if val, ok := ctx.Value(IsPlatformAdminContextKey).(bool); ok {
		return val
	}
	return false
}
