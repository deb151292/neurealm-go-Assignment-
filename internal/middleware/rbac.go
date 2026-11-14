package middleware

import (
	"fmt"
	"net/http"

	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/response"
	"github.com/neurealm/multi-tenant-system/internal/role"
)

// RequirePermission verifies the user has the required resource:action permission
func RequirePermission(roleService *role.Service, requiredPermission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if platform admin
			if domain.GetPlatformAdmin(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}

			tenantID, tok := domain.GetTenantID(r.Context())
			userID, uok := domain.GetUserID(r.Context())
			if !tok || !uok {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthenticated user", nil)
				return
			}

			// Query effective permissions (cached in memory for high-volume)
			permissions, err := roleService.GetUserPermissions(r.Context(), tenantID, userID)
			if err != nil {
				response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify permissions", nil)
				return
			}

			hasPerm := false
			for _, p := range permissions {
				if p == requiredPermission || p == "*:*" {
					hasPerm = true
					break
				}
			}

			if !hasPerm {
				// Returns 403 Forbidden as per specification
				response.Error(w, http.StatusForbidden, "FORBIDDEN",
					fmt.Sprintf("forbidden: insufficient permissions (requires '%s')", requiredPermission),
					map[string]interface{}{
						"required_permission": requiredPermission,
					},
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
