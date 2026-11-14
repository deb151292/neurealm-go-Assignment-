package middleware

import (
	"fmt"
	"net/http"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/deb151292/neurealm-go-Assignment-/internal/role"
	"github.com/gin-gonic/gin"
)

// RequirePermission verifies the user has the required resource:action permission
func RequirePermission(roleService *role.Service, requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Check if platform admin
		if domain.GetPlatformAdmin(ctx) {
			c.Next()
			return
		}

		tenantID, tok := domain.GetTenantID(ctx)
		userID, uok := domain.GetUserID(ctx)
		if !tok || !uok {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "unauthenticated user", nil)
			c.Abort()
			return
		}

		// Query effective permissions (cached in memory for high-volume)
		permissions, err := roleService.GetUserPermissions(ctx, tenantID, userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify permissions", nil)
			c.Abort()
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
			response.Error(c, http.StatusForbidden, "FORBIDDEN",
				fmt.Sprintf("forbidden: insufficient permissions (requires '%s')", requiredPermission),
				map[string]interface{}{
					"required_permission": requiredPermission,
				},
			)
			c.Abort()
			return
		}

		c.Next()
	}
}
