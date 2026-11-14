package middleware

import (
	"net/http"
	"strings"

	"github.com/neurealm/multi-tenant-system/internal/auth"
	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/response"
)

func Auth(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization header", nil)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid authorization header format, expected 'Bearer <token>'", nil)
				return
			}

			tokenStr := strings.TrimSpace(parts[1])
			claims, err := jwtService.ValidateAccessToken(tokenStr)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token", nil)
				return
			}

			// Tenant taken strictly from validated token claims
			ctx := r.Context()
			ctx = domain.WithTenantID(ctx, claims.TenantID)
			ctx = domain.WithTenantCode(ctx, claims.TenantCode)
			ctx = domain.WithUserID(ctx, claims.UserID)
			ctx = domain.WithUserEmail(ctx, claims.Email)
			ctx = domain.WithUserRoles(ctx, claims.Roles)
			ctx = domain.WithUserPermissions(ctx, claims.Permissions)
			ctx = domain.WithPlatformAdmin(ctx, claims.IsPlatformAdmin)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
