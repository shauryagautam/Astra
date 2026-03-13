package middleware

import (
	"github.com/astraframework/astra/http"
	"github.com/google/uuid"
)

// RequestID adds a unique request ID to the context and response headers.
func RequestID() http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) error {
			reqID := c.Request.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = uuid.New().String()
			}

			c.Set("request_id", reqID)
			c.Writer.Header().Set("X-Request-ID", reqID)

			return next(c)
		}
	}
}
