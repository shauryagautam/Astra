package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/shauryagautam/Astra/pkg/identity/keys"
)

type APIKeyMiddleware struct {
	manager *keys.Manager
}

func NewAPIKeyMiddleware(m *keys.Manager) *APIKeyMiddleware {
	return &APIKeyMiddleware{manager: m}
}

func (m *APIKeyMiddleware) RequireAPIKey(scope string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)
			if c == nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			key := m.extractAPIKey(r)
			if key == "" {
				c.UnauthorizedError("API key required")
				return
			}

			apiKey, err := m.manager.ValidateAPIKey(r.Context(), key)
			if err != nil {
				c.UnauthorizedError(err.Error())
				return
			}

			if scope != "" && !m.manager.CheckScope(apiKey, scope) {
				c.ForbiddenError(fmt.Sprintf("API key requires '%s' scope", scope))
				return
			}

			c.Set("api_key", apiKey)
			c.Set("user_id", apiKey.UserID)
			next.ServeHTTP(w, r)
		})
	}
}

func (m *APIKeyMiddleware) extractAPIKey(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	return r.URL.Query().Get("api_key")
}
