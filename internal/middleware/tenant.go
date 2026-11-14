package middleware

import (
	"net/http"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/gin-gonic/gin"
)

// EnforceTenantIsolation ensures that tenant context is strictly derived from the verified token
func EnforceTenantIsolation() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := domain.GetTenantID(c.Request.Context())
		if !ok || tenantID.String() == "" {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "tenant context missing in token", nil)
			c.Abort()
			return
		}

		// Add strict tenant-isolation headers to response
		c.Header("X-Tenant-Scope", tenantID.String())

		c.Next()
	}
}
