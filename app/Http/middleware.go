package http

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/shaurya/adonis/contracts"
)

// ============================================================================
// Built-in Middleware
// AdonisJS comes with CORS, Logger, and Security middleware out of the box.
// ============================================================================

// CorsMiddleware returns a middleware that handles CORS headers.
// Mirrors AdonisJS's CORS middleware from @adonisjs/cors.
func CorsMiddleware(allowedOrigins []string, allowedMethods []string, allowedHeaders []string) contracts.MiddlewareFunc {
	return func(ctx contracts.HttpContextContract, next func() error) error {
		origin := ctx.Request().Header("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			ctx.Response().Header("Access-Control-Allow-Origin", origin)
			ctx.Response().Header("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
			ctx.Response().Header("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
			ctx.Response().Header("Access-Control-Allow-Credentials", "true")
			ctx.Response().Header("Access-Control-Max-Age", "86400")
		}

		// Handle preflight
		if ctx.Request().Method() == "OPTIONS" {
			return ctx.Response().NoContent()
		}

		return next()
	}
}

// LoggerMiddleware returns a middleware that logs requests.
// Mirrors AdonisJS's Logger middleware.
func LoggerMiddleware() contracts.MiddlewareFunc {
	logger := log.New(os.Stdout, "[adonis:request] ", log.LstdFlags)
	return func(ctx contracts.HttpContextContract, next func() error) error {
		start := time.Now()
		err := next()
		duration := time.Since(start)

		status := ctx.Response().GetStatusCode()
		logger.Printf("%s %s → %d (%s)",
			ctx.Request().Method(),
			ctx.Request().URL(),
			status,
			duration.Round(time.Microsecond),
		)
		return err
	}
}

// SecureHeadersMiddleware returns a middleware that sets security headers.
// Mirrors AdonisJS's Shield middleware (@adonisjs/shield).
func SecureHeadersMiddleware() contracts.MiddlewareFunc {
	return func(ctx contracts.HttpContextContract, next func() error) error {
		ctx.Response().Header("X-Content-Type-Options", "nosniff")
		ctx.Response().Header("X-Frame-Options", "DENY")
		ctx.Response().Header("X-XSS-Protection", "1; mode=block")
		ctx.Response().Header("Referrer-Policy", "strict-origin-when-cross-origin")
		ctx.Response().Header("X-Download-Options", "noopen")
		ctx.Response().Header("X-Permitted-Cross-Domain-Policies", "none")
		return next()
	}
}

// RecoveryMiddleware returns a middleware that recovers from panics.
func RecoveryMiddleware() contracts.MiddlewareFunc {
	logger := log.New(os.Stderr, "[adonis:panic] ", log.LstdFlags)
	return func(ctx contracts.HttpContextContract, next func() error) error {
		defer func() {
			if r := recover(); r != nil {
				logger.Printf("PANIC: %v", r)
				ctx.Response().Status(500).Json(map[string]any{ //nolint:errcheck
					"error":   "Internal Server Error",
					"message": "An unexpected error occurred",
					"status":  500,
				})
			}
		}()
		return next()
	}
}
