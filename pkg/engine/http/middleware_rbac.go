package http

import (
	"log/slog"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/identity/rbac"
)

// RBACMiddleware provides RBAC middleware for HTTP handlers
type RBACMiddleware struct {
	rbac   *rbac.RBAC
	logger *slog.Logger
}

// NewRBACMiddleware creates new RBAC middleware
func NewRBACMiddleware(r *rbac.RBAC, logger *slog.Logger) *RBACMiddleware {
	return &RBACMiddleware{
		rbac:   r,
		logger: logger,
	}
}

// RequirePermission creates middleware that requires specific permission.
func (m *RBACMiddleware) RequirePermission(resource, action string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)
			if c == nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			user := c.AuthUser()
			if user == nil {
				c.UnauthorizedError("authentication required")
				return
			}

			req := &rbac.AccessRequest{
				UserID:   user.UserID,
				Resource: resource,
				Action:   action,
				Context:  buildContext(c, r),
			}

			result, err := m.rbac.CheckAccess(r.Context(), req)
			if err != nil {
				m.logger.Error("RBAC check failed", "error", err, "user_id", user.UserID)
				c.InternalError("RBAC check failed")
				return
			}

			if !result.Allowed {
				c.ForbiddenError(result.Reason)
				return
			}

			c.Set("rbac_result", result)
			next.ServeHTTP(w, r)
		})
	}
}

func buildContext(c *Context, r *http.Request) map[string]interface{} {
	return map[string]interface{}{
		"method":     r.Method,
		"path":       r.URL.Path,
		"ip":         c.ClientIP(),
		"user_agent": r.UserAgent(),
	}
}
