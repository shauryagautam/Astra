package http // Astra CSRF Protection Middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	csrfCookieName = "astra_csrf"
	csrfHeaderName = "X-CSRF-Token"
	spaHeaderName  = "X-Requested-With"
)

type CSRFMode int

const (
	CSRFModeAuto CSRFMode = iota // Automatically detect client type
	CSRFModeSPA                  // SPA/Vite frontend (cookie-to-header token reflection)
	CSRFModeAPI                  // API clients (stateless JWTs, skip CSRF)
)

type CSRFConfig struct {
	Mode               CSRFMode
	CookieName         string
	HeaderName         string
	SPAHeaderName      string
	SecureCookie       bool
	CookieHTTPOnly      bool // If true, the JS cannot read the cookie (recommended for most, except SPA reflection)
	SameSitePolicy     http.SameSite
	ExemptPaths        []string
	ExemptMethods      []string
	TokenExpiry        time.Duration
	IsProd             bool // Explicit dependency
}

func DefaultCSRFConfig() *CSRFConfig {
	return &CSRFConfig{
		Mode:           CSRFModeAuto,
		CookieName:     csrfCookieName,
		HeaderName:     csrfHeaderName,
		SPAHeaderName:  spaHeaderName,
		SecureCookie:   true,
		CookieHTTPOnly: true,
		SameSitePolicy: http.SameSiteLaxMode,
		ExemptMethods:  []string{"GET", "HEAD", "OPTIONS"},
		TokenExpiry:    24 * time.Hour,
		IsProd:         false,
	}
}

// CSRF implements Cross-Site Request Forgery protection with smart client detection.
func CSRF(isProd bool, opts ...func(*CSRFConfig)) MiddlewareFunc {
	config := DefaultCSRFConfig()
	config.IsProd = isProd
	for _, opt := range opts {
		opt(config)
	}

	// Apply SPA defaults if in SPA mode
	if config.Mode == CSRFModeSPA {
		if config.CookieName == csrfCookieName {
			config.CookieName = "XSRF-TOKEN"
		}
		if config.HeaderName == csrfHeaderName {
			config.HeaderName = "X-XSRF-Token"
		}
		// In SPA mode, we allow JS to read the cookie for reflection
		config.CookieHTTPOnly = false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)
			
			// Sync existing cookie token to context if present
			if cookie, err := r.Cookie(config.CookieName); err == nil {
				if c != nil {
					c.Set("astra_csrf_token", cookie.Value)
				}
			}

			// Determine client type
			clientType := detectClientType(r, c, config)

			// 1. If not a programmatic API client, ensure CSRF cookie is present
			if clientType != "api" {
				if err := ensureCSRFCookie(w, r, c, config); err != nil {
					if c != nil {
						c.InternalError(err.Error())
					} else {
						http.Error(w, err.Error(), http.StatusInternalServerError)
					}
					return
				}
			}

			// 2. Check if request should be exempted from verification
			if isExemptPath(r.URL.Path, config.ExemptPaths) ||
				isExemptMethod(r.Method, config.ExemptMethods) {
				next.ServeHTTP(w, r)
				return
			}

			// 3. Perform CSRF verification
			switch clientType {
			case "api":
				// API clients: skip ONLY if actually authenticated via stateless method (JWT/API Key)
				if isAuthenticatedStateless(c, r) {
					next.ServeHTTP(w, r)
					return
				}
				// Default to CSRF check for unauthenticated or session-based API requests
			case "spa":
				// SPA/Vite: verified via cookie-to-header reflection
				if err := verifyCSRFToken(w, r, c, config); err != nil {
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Default: apply CSRF protection
			if err := verifyCSRFToken(w, r, c, config); err != nil {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions for enhanced CSRF protection

func detectClientType(r *http.Request, _ *Context, config *CSRFConfig) string {
	switch config.Mode {
	case CSRFModeSPA:
		return "spa"
	case CSRFModeAPI:
		return "api"
	case CSRFModeAuto:
		// Auto-detect based on request characteristics
		accept := r.Header.Get("Accept")
		userAgent := r.Header.Get("User-Agent")
		spaHeader := r.Header.Get(config.SPAHeaderName)
		authHeader := r.Header.Get("Authorization")

		// SPA indicators
		isSPA := spaHeader == "XMLHttpRequest" ||
			((strings.Contains(userAgent, "Mozilla") || strings.Contains(userAgent, "Chrome") || strings.Contains(userAgent, "Safari")) &&
				(strings.Contains(accept, "application/json") || strings.Contains(accept, "text/html")) &&
				!strings.HasPrefix(authHeader, "Bearer "))

		// API indicators
		isAPI := strings.HasPrefix(authHeader, "Bearer ") ||
			r.Header.Get("X-API-Key") != "" ||
			strings.Contains(accept, "application/vnd.api+")

		if isAPI {
			return "api"
		}
		if isSPA {
			return "spa"
		}

		// Default to SPA for web-like requests if still unsure
		if strings.Contains(accept, "text/html") || strings.Contains(accept, "application/json") {
			return "spa"
		}
	}

	return "unknown"
}

func isAuthenticatedStateless(c *Context, r *http.Request) bool {
	if c == nil {
		return false
	}
	// Check for JWT authentication
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return c.IsAuthenticated()
	}

	// Check for API key authentication
	if r.Header.Get("X-API-Key") != "" {
		return c.IsAuthenticated()
	}

	return false
}

func isExemptPath(path string, exemptPaths []string) bool {
	for _, exemptPath := range exemptPaths {
		if strings.HasPrefix(path, exemptPath) {
			return true
		}
	}
	return false
}

func isExemptMethod(method string, exemptMethods []string) bool {
	for _, exemptMethod := range exemptMethods {
		if method == exemptMethod {
			return true
		}
	}
	return false
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request, c *Context, config *CSRFConfig) error {
	if _, err := r.Cookie(config.CookieName); err != nil {
		return setCSRFCookieWithConfig(w, r, c, config)
	}
	return nil
}

func verifyCSRFToken(w http.ResponseWriter, r *http.Request, c *Context, config *CSRFConfig) error {
	cookie, err := r.Cookie(config.CookieName)
	if err != nil {
		if c != nil {
			return c.Error(http.StatusForbidden, "CSRF token cookie is missing")
		}
		http.Error(w, "CSRF token cookie is missing", http.StatusForbidden)
		return fmt.Errorf("CSRF token cookie is missing")
	}

	headerToken := r.Header.Get(config.HeaderName)
	formToken := r.FormValue("_csrf")
	if formToken == "" {
		formToken = r.FormValue("astra_csrf")
	}

	token := headerToken
	if token == "" {
		token = formToken
	}

	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
		if c != nil {
			return c.Error(http.StatusForbidden, "CSRF token is invalid or missing")
		}
		http.Error(w, "CSRF token is invalid or missing", http.StatusForbidden)
		return fmt.Errorf("CSRF token is invalid or missing")
	}

	return nil
}

func setCSRFCookieWithConfig(w http.ResponseWriter, _ *http.Request, c *Context, config *CSRFConfig) error {
	token, err := generateRandomToken(32)
	if err != nil {
		return err
	}

	secure := config.SecureCookie
	if secure {
		secure = config.IsProd
	}

	// Always store in context for template helpers even if just generated
	if c != nil {
		c.Set("astra_csrf_token", token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     config.CookieName,
		Value:    token,
		Expires:  time.Now().Add(config.TokenExpiry),
		HttpOnly: config.CookieHTTPOnly,
		Secure:   secure,
		SameSite: config.SameSitePolicy,
		Path:     "/",
	})
	return nil
}

// CSRF configuration options

// WithCSRFMode sets the CSRF protection mode (auto, spa, api)
func WithCSRFMode(mode CSRFMode) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.Mode = mode
	}
}

// WithCSRFCookieName sets a custom CSRF cookie name
func WithCSRFCookieName(name string) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.CookieName = name
	}
}

// WithCSRFHeaderName sets a custom CSRF header name
func WithCSRFHeaderName(name string) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.HeaderName = name
	}
}

// WithCSRFSPAHeaderName sets the SPA detection header name
func WithCSRFSPAHeaderName(name string) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.SPAHeaderName = name
	}
}

// WithCSRFSecureCookie controls whether the CSRF cookie should be secure
func WithCSRFSecureCookie(secure bool) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.SecureCookie = secure
	}
}

// WithCSRFCookieHTTPOnly controls whether the CSRF cookie should be HttpOnly
func WithCSRFCookieHTTPOnly(httpOnly bool) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.CookieHTTPOnly = httpOnly
	}
}

// WithCSRFSameSite sets the SameSite policy for CSRF cookies
func WithCSRFSameSite(policy http.SameSite) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.SameSitePolicy = policy
	}
}

// WithCSRFExemptPaths sets paths that should be exempt from CSRF protection
func WithCSRFExemptPaths(paths ...string) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.ExemptPaths = paths
	}
}

// WithCSRFExemptMethods sets HTTP methods that should be exempt from CSRF protection
func WithCSRFExemptMethods(methods ...string) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.ExemptMethods = methods
	}
}

// WithCSRFTokenExpiry sets the expiration time for CSRF tokens
func WithCSRFTokenExpiry(expiry time.Duration) func(*CSRFConfig) {
	return func(cfg *CSRFConfig) {
		cfg.TokenExpiry = expiry
	}
}

func generateRandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
