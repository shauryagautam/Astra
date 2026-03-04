package middleware

import (
	"log/slog"
	"time"

	"github.com/astraframework/astra/http"
)

// Logger returns a middleware that logs HTTP requests.
func Logger(logger *slog.Logger) http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) error {
			start := time.Now()
			
			// Let the request proceed
			err := next(c)
			
			duration := time.Since(start)
			reqID := c.GetString("request_id")
			
			logger.Info("HTTP Request",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"ip", c.ClientIP(),
				"duration", duration.String(),
				"request_id", reqID,
			)
			
			return err
		}
	}
}
