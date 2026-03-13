package http

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
)

// Recover returns a middleware that recovers from panics and returns a 500 error.
func Recover(logger *slog.Logger) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if logger != nil {
						logger.Error("panic recovered",
							"error", r,
							"stack", string(debug.Stack()),
						)
					}
					err = NewHTTPError(500, "INTERNAL_ERROR", "An unexpected error occurred")
				}
			}()

			return next(c)
		}
	}
}

// RequestID returns a middleware that injects a unique request ID into the context and response headers.
func RequestID() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			id := c.Request.Header.Get("X-Request-ID")
			if id == "" {
				id = uuid.NewString()
			}

			c.Set("request_id", id)
			c.Writer.Header().Set("X-Request-ID", id)

			return next(c)
		}
	}
}

// Logger returns a middleware that logs incoming requests.
func Logger(logger *slog.Logger) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			if logger == nil {
				return next(c)
			}

			start := time.Now()
			path := c.Request.URL.Path
			method := c.Request.Method

			err := next(c)

			status := c.Status()
			duration := time.Since(start)

			msg := fmt.Sprintf("%d %s %s", status, method, path)

			attrs := []any{
				slog.Int("status", status),
				slog.String("method", method),
				slog.String("path", path),
				slog.Duration("duration", duration),
				slog.String("ip", c.Request.RemoteAddr),
			}

			if reqID := c.Get("request_id"); reqID != nil {
				attrs = append(attrs, slog.Any("request_id", reqID))
			}

			if err != nil {
				attrs = append(attrs, slog.Any("error", err))
				logger.Error(msg, attrs...)
			} else {
				logger.Info(msg, attrs...)
			}

			return nil
		}
	}
}
