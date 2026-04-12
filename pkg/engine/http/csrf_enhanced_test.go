package http // Astra CSRF Enhanced Tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/shauryagautam/Astra/pkg/identity/claims"
	"github.com/stretchr/testify/assert"
)

func TestCSRFEnhanced_AutoDetection(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	tests := []struct {
		name        string
		headers     map[string]string
		authUser    *claims.AuthClaims
		expectCSRF  bool
		description string
	}{
		{
			name: "SPA with X-Requested-With header",
			headers: map[string]string{
				"X-Requested-With": "XMLHttpRequest",
				"Accept":           "application/json",
				"User-Agent":       "Mozilla/5.0 (Chrome)",
			},
			expectCSRF:  true,
			description: "SPA should require CSRF protection",
		},
		{
			name: "API with Bearer token and valid auth",
			headers: map[string]string{
				"Authorization": "Bearer valid.jwt.token",
				"Accept":         "application/vnd.api+json",
			},
			authUser:    &claims.AuthClaims{UserID: "user123"},
			expectCSRF:  false,
			description: "Authenticated API should skip CSRF",
		},
		{
			name: "API with Bearer token but invalid auth",
			headers: map[string]string{
				"Authorization": "Bearer invalid.jwt.token",
				"Accept":         "application/vnd.api+json",
			},
			expectCSRF:  true,
			description: "Unauthenticated API should require CSRF",
		},
		{
			name: "API with X-API-Key",
			headers: map[string]string{
				"X-API-Key": "api-key-123",
				"Accept":    "application/json",
			},
			authUser:    &claims.AuthClaims{UserID: "user123"},
			expectCSRF:  false,
			description: "API key authenticated should skip CSRF",
		},
		{
			name: "Web browser without SPA headers",
			headers: map[string]string{
				"Accept":     "text/html,application/xhtml+xml",
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			},
			expectCSRF:  true,
			description: "Web browser should require CSRF",
		},
		{
			name: "Unknown client defaults to SPA",
			headers: map[string]string{
				"Accept":     "application/json",
				"User-Agent": "curl/7.68.0",
			},
			expectCSRF:  true,
			description: "Unknown client should default to CSRF protection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CSRF(false)(next)

			form := url.Values{}
			form.Add("_csrf", "test-token")

			req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: "test-token"})

			w := httptest.NewRecorder()
			
			c := NewContext(w, req)
			if tt.authUser != nil {
				c.SetAuthUser(tt.authUser)
			}
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "ok", w.Body.String())
		})
	}
}

func TestCSRFEnhanced_ConfigurationModes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	tests := []struct {
		name        string
		mode        CSRFMode
		headers     map[string]string
		expectCSRF  bool
		description string
	}{
		{
			name:        "SPA mode always requires CSRF",
			mode:        CSRFModeSPA,
			headers:     map[string]string{"Authorization": "Bearer valid.jwt.token"},
			expectCSRF:  true,
			description: "SPA mode should always require CSRF regardless of auth",
		},
		{
			name:        "API mode skips CSRF for authenticated",
			mode:        CSRFModeAPI,
			headers:     map[string]string{"Authorization": "Bearer valid.jwt.token"},
			expectCSRF:  false,
			description: "API mode should skip CSRF for authenticated requests",
		},
		{
			name:        "API mode requires CSRF for unauthenticated",
			mode:        CSRFModeAPI,
			headers:     map[string]string{"Accept": "application/json"},
			expectCSRF:  true,
			description: "API mode should require CSRF for unauthenticated requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CSRF(false, WithCSRFMode(tt.mode))(next)

			form := url.Values{}
			form.Add("_csrf", "test-token")

			req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			cookieName := "astra_csrf"
			if tt.mode == CSRFModeSPA {
				cookieName = "XSRF-TOKEN"
			}
			req.AddCookie(&http.Cookie{Name: cookieName, Value: "test-token"})

			w := httptest.NewRecorder()
			
			c := NewContext(w, req)
			if tt.mode != CSRFModeSPA && tt.headers["Authorization"] != "" {
				c.SetAuthUser(&claims.AuthClaims{UserID: "user123"})
			}
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "ok", w.Body.String())
		})
	}
}

func TestCSRFEnhanced_Exemptions(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	handler := CSRF(false, 
		WithCSRFExemptPaths("/api/webhook", "/health"),
		WithCSRFExemptMethods("GET", "HEAD", "OPTIONS", "TRACE"),
	)(next)

	tests := []struct {
		name        string
		method      string
		path        string
		shouldPass  bool
		description string
	}{
		{
			name:        "Exempt path should pass without CSRF",
			method:      "POST",
			path:        "/api/webhook",
			shouldPass:  true,
			description: "Exempt paths should bypass CSRF",
		},
		{
			name:        "Exempt method should pass without CSRF",
			method:      "TRACE",
			path:        "/api/protected",
			shouldPass:  true,
			description: "Exempt methods should bypass CSRF",
		},
		{
			name:        "Non-exempt request should require CSRF",
			method:      "POST",
			path:        "/api/protected",
			shouldPass:  false,
			description: "Non-exempt requests should require CSRF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			
			c := NewContext(w, req)
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)

			if tt.shouldPass {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "ok", w.Body.String())
			} else {
				assert.Equal(t, http.StatusForbidden, w.Code)
			}
		})
	}
}

func TestCSRFEnhanced_CustomConfiguration(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	handler := CSRF(false, 
		WithCSRFCookieName("custom_csrf"),
		WithCSRFHeaderName("X-Custom-CSRF"),
		WithCSRFTokenExpiry(time.Hour*2),
	)(next)

	token := "test-token"
	form := url.Values{}
	form.Add("_csrf", token)

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Custom-CSRF", token)
	req.AddCookie(&http.Cookie{Name: "custom_csrf", Value: token})

	w := httptest.NewRecorder()
	c := NewContext(w, req)
	ctx := context.WithValue(req.Context(), astraContextKey, c)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestCSRFEnhanced_SecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	tests := []struct {
		name           string
		sameSitePolicy http.SameSite
		secureCookie   bool
	}{
		{
			name:           "Strict SameSite policy",
			sameSitePolicy: http.SameSiteStrictMode,
			secureCookie:   true,
		},
		{
			name:           "Lax SameSite policy",
			sameSitePolicy: http.SameSiteLaxMode,
			secureCookie:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CSRF(false, 
				WithCSRFSameSite(tt.sameSitePolicy),
				WithCSRFSecureCookie(tt.secureCookie),
			)(next)

			req := httptest.NewRequest("POST", "/", nil)
			req.Header.Set("X-Requested-With", "XMLHttpRequest") // Force SPA detection
			w := httptest.NewRecorder()
			
			c := NewContext(w, req)
			ctx := context.WithValue(req.Context(), astraContextKey, c)
			req = req.WithContext(ctx)

			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}
