package http

import (
	"net/http"
	"time"
)

// Timeout returns a standard middleware that wraps the handler with http.TimeoutHandler.
func Timeout(timeout time.Duration) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, "Request timed out")
	}
}
