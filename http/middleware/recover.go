package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/astraframework/astra/http"
)

// Recover returns a middleware that recovers from panics and returns a 500 error.
func Recover(logger *slog.Logger) http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("panic recovered",
						"error", r,
						"stack", string(debug.Stack()),
					)
					err = http.NewHTTPError(500, "INTERNAL_ERROR", "An unexpected error occurred")
				}
			}()
			
			return next(c)
		}
	}
}
