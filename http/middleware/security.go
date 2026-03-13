package middleware

import (
	astrahttp "github.com/astraframework/astra/http"
)

// SecurityHeadersConfig defines the configuration for the SecurityHeaders middleware.
type SecurityHeadersConfig struct {
	ContentSecurityPolicy   string
	XFrameOptions           string
	XContentTypeOptions     string
	StrictTransportSecurity string
	ReferrerPolicy          string
	PermissionsPolicy       string
}

// DefaultSecurityHeadersConfig returns the default configuration for production-grade security.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		ContentSecurityPolicy:   "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'self';",
		XFrameOptions:           "DENY",
		XContentTypeOptions:     "nosniff",
		StrictTransportSecurity: "max-age=31536000; includeSubDomains",
		ReferrerPolicy:          "no-referrer-when-downgrade",
		PermissionsPolicy:       "geolocation=(), microphone=(), camera=()",
	}
}

// SecurityHeaders returns a middleware that sets standard security-related HTTP headers.
func SecurityHeaders(cfg ...SecurityHeadersConfig) astrahttp.MiddlewareFunc {
	config := DefaultSecurityHeadersConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}

	return func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
		return func(c *astrahttp.Context) error {
			if config.ContentSecurityPolicy != "" {
				c.SetHeader("Content-Security-Policy", config.ContentSecurityPolicy)
			}
			if config.XFrameOptions != "" {
				c.SetHeader("X-Frame-Options", config.XFrameOptions)
			}
			if config.XContentTypeOptions != "" {
				c.SetHeader("X-Content-Type-Options", config.XContentTypeOptions)
			}
			if config.StrictTransportSecurity != "" {
				c.SetHeader("Strict-Transport-Security", config.StrictTransportSecurity)
			}
			if config.ReferrerPolicy != "" {
				c.SetHeader("Referrer-Policy", config.ReferrerPolicy)
			}
			if config.PermissionsPolicy != "" {
				c.SetHeader("Permissions-Policy", config.PermissionsPolicy)
			}

			return next(c)
		}
	}
}
