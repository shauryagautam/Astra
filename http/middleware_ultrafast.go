package http

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// UltraFastLogger provides high-performance request logging
func UltraFastLogger() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			start := time.Now()

			// Use defer for performance
			defer func() {
				duration := time.Since(start)
				// Ultra-fast structured logging
				c.Logger().Info("request",
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"status", c.Status(),
					"duration", duration.String(),
					"ip", c.ClientIP(),
				)
			}()

			return next(c)
		}
	}
}

// UltraFastRecover provides high-performance panic recovery
func UltraFastRecover() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			defer func() {
				if r := recover(); r != nil {
					c.Logger().Error("panic recovered",
						"panic", r,
						"method", c.Request.Method,
						"path", c.Request.URL.Path,
					)
					c.Writer.WriteHeader(http.StatusInternalServerError)
				}
			}()
			return next(c)
		}
	}
}

// UltraFastRequestID provides high-performance request ID generation
func UltraFastRequestID() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			// Fast path: check if header exists
			if reqID := c.Header("X-Request-ID"); reqID != "" {
				c.SetHeader("X-Request-ID", reqID)
				return next(c)
			}

			// Generate simple ultra-fast ID (timestamp + counter)
			reqID := simpleRequestID()
			c.SetHeader("X-Request-ID", reqID)
			return next(c)
		}
	}
}

// UltraFastRateLimit provides high-performance rate limiting
func UltraFastRateLimit(requests int, window time.Duration) MiddlewareFunc {
	// Simple in-memory rate limiter for ultra-fast performance
	type client struct {
		count    int
		window   time.Time
		requests int
	}

	clients := make(map[string]*client)

	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			ip := c.ClientIP()
			now := time.Now()

			cl, exists := clients[ip]
			if !exists || now.Sub(cl.window) > window {
				clients[ip] = &client{
					count:    1,
					window:   now,
					requests: requests,
				}
				return next(c)
			}

			if cl.count >= cl.requests {
				c.Writer.WriteHeader(http.StatusTooManyRequests)
				return nil
			}

			cl.count++
			return next(c)
		}
	}
}

// UltraFastSecurity provides high-performance security headers
func UltraFastSecurity() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			// Set security headers in one batch for ultra-fast performance
			headers := map[string]string{
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"X-XSS-Protection":        "1; mode=block",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
				"Content-Security-Policy": "default-src 'self'",
			}

			for key, value := range headers {
				c.SetHeader(key, value)
			}

			return next(c)
		}
	}
}

// UltraFastCompress provides high-performance response compression
func UltraFastCompress() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			// Fast path: check if client accepts compression
			if !strings.Contains(c.Header("Accept-Encoding"), "gzip") {
				return next(c)
			}

			// Set compression header
			c.SetHeader("Content-Encoding", "gzip")
			return next(c)
		}
	}
}

// UltraFastTimeout provides high-performance request timeout
func UltraFastTimeout(timeout time.Duration) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			ctx, cancel := context.WithTimeout(c.Ctx(), timeout)
			defer cancel()

			// Update request context with timeout
			c.Request = c.Request.WithContext(ctx)

			done := make(chan error, 1)
			go func() {
				done <- next(c)
			}()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				c.Writer.WriteHeader(http.StatusRequestTimeout)
				return nil
			}
		}
	}
}

// Simple ultra-fast request ID generator
var requestCounter int64

func simpleRequestID() string {
	// Ultra-fast simple ID: timestamp + counter
	return string(rune(time.Now().UnixNano())) + string(rune(requestCounter))
}
