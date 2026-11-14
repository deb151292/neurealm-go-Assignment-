package middleware

import (
	"net/http"

	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/response"
)

// EnforceTenantIsolation ensures that tenant context is strictly derived from the verified token
func EnforceTenantIsolation() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, ok := domain.GetTenantID(r.Context())
			if !ok || tenantID.String() == "" {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "tenant context missing in token", nil)
				return
			}

			// Add strict tenant-isolation headers to response
			w.Header().Set("X-Tenant-Scope", tenantID.String())

			next.ServeHTTP(w, r)
		})
	}
}
