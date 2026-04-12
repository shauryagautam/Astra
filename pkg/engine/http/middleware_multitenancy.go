package http

import (
	"log/slog"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/identity/multitenancy"
)

// TenantMiddleware handles multi-tenancy resolution and access control.
type TenantMiddleware struct {
	manager  *multitenancy.Manager
	resolver *multitenancy.TenantResolver
	logger   *slog.Logger
}

func NewTenantMiddleware(m *multitenancy.Manager, logger *slog.Logger) *TenantMiddleware {
	return &TenantMiddleware{
		manager:  m,
		resolver: multitenancy.NewTenantResolver(m, multitenancy.Config{}), // Simplified
		logger:   logger,
	}
}

func (m *TenantMiddleware) RequireTenant() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)
			if c == nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			tenant, err := m.resolver.ResolveTenant(r.Context(), r.Host, r.URL.Path)
			if err != nil {
				c.NotFoundError("Tenant")
				return
			}

			c.Set("tenant", tenant)
			c.Set("tenant_id", tenant.ID)
			next.ServeHTTP(w, r)
		})
	}
}
