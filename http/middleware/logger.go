package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/astraframework/astra/http"
)

var defaultSensitiveKeys = []string{
	"password", "passwd", "secret", "token", "auth", "api_key", "apikey",
	"access_token", "refresh_token", "session", "csrf", "credit_card",
	"card_number", "cvv", "ssn",
}

// Logger returns a middleware that logs HTTP requests with PII redaction.
func Logger(logger *slog.Logger) http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) error {
			start := time.Now()

			// Let the request proceed
			err := next(c)

			duration := time.Since(start)

			path := c.Request.URL.Path
			// Redact sensitive info from path if any (unlikely but safe)
			for _, key := range defaultSensitiveKeys {
				if strings.Contains(strings.ToLower(path), key+"=") {
					// Very basic path param redaction (e.g. /reset?token=...)
					// Better to encourage developers not to put PII in URLs
					path = "[REDACTED]"
					break
				}
			}

			c.Logger().Info("HTTP Request",
				"method", c.Request.Method,
				"path", path,
				"ip", c.ClientIP(),
				"duration", duration.String(),
				"status", c.Status(),
			)

			return err
		}
	}
}
