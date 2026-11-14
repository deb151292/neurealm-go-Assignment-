package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/deb151292/neurealm-go-Assignment-/internal/auth"
	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
)

func Auth(jwtService *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization header", nil)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid authorization header format, expected 'Bearer <token>'", nil)
			c.Abort()
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		claims, err := jwtService.ValidateAccessToken(tokenStr)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired access token", nil)
			c.Abort()
			return
		}

		// Inject into both Gin context and standard request context
		ctx := c.Request.Context()
		ctx = domain.WithTenantID(ctx, claims.TenantID)
		ctx = domain.WithTenantCode(ctx, claims.TenantCode)
		ctx = domain.WithUserID(ctx, claims.UserID)
		ctx = domain.WithUserEmail(ctx, claims.Email)
		ctx = domain.WithUserRoles(ctx, claims.Roles)
		ctx = domain.WithUserPermissions(ctx, claims.Permissions)
		ctx = domain.WithPlatformAdmin(ctx, claims.IsPlatformAdmin)

		c.Request = c.Request.WithContext(ctx)

		c.Set("tenant_id", claims.TenantID)
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("is_platform_admin", claims.IsPlatformAdmin)

		c.Next()
	}
}
