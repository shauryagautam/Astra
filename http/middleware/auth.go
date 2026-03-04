package middleware

import (
	"github.com/astraframework/astra/auth"
	"github.com/astraframework/astra/http"
)

// Auth returns a middleware that protects routes using the provided guard.
func Auth(guard auth.Guard) http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) error {
			if err := guard.Attempt(c); err != nil {
				return http.ErrUnauthorized.WithMessage(err.Error())
			}
			return next(c)
		}
	}
}
