package http

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// SecurityConfig defines the configuration for secure headers.
type SecurityConfig struct {
	XSSProtection         string
	ContentTypeOptions    string
	FrameOptions          string
	ReferrerPolicy        string
	ContentSecurityPolicy string
	PermissionsPolicy     string
	HSTSMaxAge            int
	HSTSPreload           bool
	HSTSIncludeSubdomains bool
}

// DefaultSSRSecurityConfig returns the recommended security defaults for server-rendered HTML applications.
func DefaultSSRSecurityConfig() SecurityConfig {
	return SecurityConfig{
		XSSProtection:         "0",
		ContentTypeOptions:    "nosniff",
		FrameOptions:          "SAMEORIGIN",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; object-src 'none';",
		PermissionsPolicy:     "camera=(), microphone=(), geolocation=()",
		HSTSMaxAge:            31536000, // 1 year
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
	}
}

// DefaultAPISecurityConfig returns the recommended security defaults for API endpoints.
func DefaultAPISecurityConfig() SecurityConfig {
	return SecurityConfig{
		XSSProtection:         "0",
		ContentTypeOptions:    "nosniff",
		FrameOptions:          "DENY",
		ReferrerPolicy:        "no-referrer",
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none';",
		PermissionsPolicy:     "camera=(), microphone=(), geolocation=()",
		HSTSMaxAge:            31536000, // 1 year
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
	}
}

// DefaultSecurityConfig is an alias for DefaultSSRSecurityConfig for backwards compatibility.
func DefaultSecurityConfig() SecurityConfig {
	return DefaultSSRSecurityConfig()
}

// SecureHeaders returns a standard middleware that sets common security headers.
func SecureHeaders(isProd bool, config ...SecurityConfig) MiddlewareFunc {
	cfg := DefaultSecurityConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)

			if cfg.XSSProtection != "" {
				w.Header().Set("X-XSS-Protection", cfg.XSSProtection)
			} else {
				w.Header().Del("X-XSS-Protection")
			}

			if cfg.ContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", cfg.ContentTypeOptions)
			} else {
				w.Header().Del("X-Content-Type-Options")
			}

			if cfg.FrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.FrameOptions)
			} else {
				w.Header().Del("X-Frame-Options")
			}

			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			} else {
				w.Header().Del("Referrer-Policy")
			}

			if cfg.ContentSecurityPolicy != "" {
				csp := cfg.ContentSecurityPolicy
				if c != nil {
					if nonce := c.GetString("csp_nonce"); nonce != "" {
						csp = strings.ReplaceAll(csp, "{nonce}", "'nonce-"+nonce+"'")
					}
				}
				w.Header().Set("Content-Security-Policy", csp)
			} else {
				w.Header().Del("Content-Security-Policy")
			}

			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			} else {
				w.Header().Del("Permissions-Policy")
			}

			// HSTS only in production and over HTTPS
			if isProd && cfg.HSTSMaxAge > 0 {
				hsts := fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
				if cfg.HSTSIncludeSubdomains {
					hsts += "; includeSubDomains"
				}
				if cfg.HSTSPreload {
					hsts += "; preload"
				}
				w.Header().Set("Strict-Transport-Security", hsts)
			} else {
				w.Header().Del("Strict-Transport-Security")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NonceMiddleware generates a random nonce for CSP and stores it in the context.
func NonceMiddleware() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)
			if c == nil {
				next.ServeHTTP(w, r)
				return
			}

			nonce := make([]byte, 16)
			if _, err := rand.Read(nonce); err != nil {
				next.ServeHTTP(w, r)
				return
			}
			c.Set("csp_nonce", base64.StdEncoding.EncodeToString(nonce))
			next.ServeHTTP(w, r)
		})
	}
}
