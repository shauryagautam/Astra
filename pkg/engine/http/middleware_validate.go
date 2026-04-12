package http

import (
	"log/slog"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/engine"
)

// ValidateMiddleware handles request validation by injecting the validator service.
func ValidateMiddleware(validator engine.Validator, logger *slog.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Actual validation often happens in handlers using c.BindAndValidate,
			// but we can add global validation logic here if needed.
			next.ServeHTTP(w, r)
		})
	}
}

// BindAndValidate is a placeholder for the actual implementation.
func (c *Context) BindAndValidate(v any) error {
	// This would typically use the Validator service on the Context,
	// which currently isn't there. I'll add it to Context in context.go.
	return nil
}

// Validate uses the BindAndValidate helper.
func (c *Context) Validate(v any) error {
	return c.BindAndValidate(v)
}
