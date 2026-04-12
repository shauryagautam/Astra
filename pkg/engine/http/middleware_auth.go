package http

import (
	"net/http"

	"github.com/shauryagautam/Astra/pkg/identity/auth"
)

// Auth returns a standard middleware that protects routes using the provided guard.
func Auth(guard auth.Guard) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)
			if c == nil {
				// Fallback if Context is missing
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if err := guard.Attempt(c); err != nil {
				c.UnauthorizedError(err.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
