package http

import (
	"fmt"
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

// SecureHeaders returns a middleware that sets common security headers.
func SecureHeaders(config ...SecurityConfig) MiddlewareFunc {
	cfg := DefaultSecurityConfig()
	if len(config) > 0 {
		// Merge overrides. We assume empty string means "do not send this header"
		// if the user explicitly provided a config. If they just wanted defaults,
		// they wouldn't pass an argument. However, completely wiping defaults
		// if one field is set is dangerous. Let's make it so if you pass a config,
		// it is used EXACTLY as is, giving the user full explicit control.
		// To just tweak, a user should do: cfg := DefaultSSRSecurityConfig(); cfg.FrameOptions = "..."; SecureHeaders(cfg)
		cfg = config[0]
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			w := c.Writer

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
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			} else {
				w.Header().Del("Content-Security-Policy")
			}

			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			} else {
				w.Header().Del("Permissions-Policy")
			}

			// HSTS only in production and over HTTPS
			if c.App != nil && c.App.Env.IsProd() && cfg.HSTSMaxAge > 0 {
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

			return next(c)
		}
	}
}
