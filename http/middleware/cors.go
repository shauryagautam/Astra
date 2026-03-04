package middleware

import (
	"fmt"
	"net/http"
	"strings"

	astrahttp "github.com/astraframework/astra/http"
)

// CorsConfig defines the CORS configuration.
type CorsConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int  // Cache duration for preflight in seconds (Access-Control-Max-Age)
	Strict           bool // If true, return 403 for disallowed origins (default: pass through)
}

// DefaultCors returns a permissive CORS config suitable for local development.
func DefaultCors() CorsConfig {
	return CorsConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposeHeaders:    []string{},
		AllowCredentials: false,
		MaxAge:           86400,
	}
}

// CORS returns a middleware that handles CORS requests.
func CORS(config CorsConfig) astrahttp.MiddlewareFunc {
	return func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
		return func(c *astrahttp.Context) error {
			origin := c.Request.Header.Get("Origin")

			// Always add Vary: Origin so caches don't serve wrong CORS headers
			c.Writer.Header().Add("Vary", "Origin")

			if origin == "" {
				return next(c)
			}

			// Check if origin is allowed
			allowed := false
			for _, o := range config.AllowOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if !allowed {
				if config.Strict {
					c.Writer.WriteHeader(http.StatusForbidden)
					return nil
				}
				return next(c)
			}

			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)

			if config.AllowCredentials {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if len(config.AllowHeaders) > 0 {
				c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
			}
			if len(config.AllowMethods) > 0 {
				c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
			}
			if len(config.ExposeHeaders) > 0 {
				c.Writer.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
			}

			// Preflight request
			if c.Request.Method == "OPTIONS" {
				if config.MaxAge > 0 {
					c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
				}
				c.Writer.WriteHeader(204)
				return nil
			}

			return next(c)
		}
	}
}
