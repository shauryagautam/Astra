package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureHeaders_DefaultSSR(t *testing.T) {
	router := NewRouter(nil)

	router.Use(SecureHeaders(DefaultSSRSecurityConfig()))
	router.Get("/", func(c *Context) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	headers := rec.Header()
	assert.Equal(t, "0", headers.Get("X-XSS-Protection"))
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Equal(t, "SAMEORIGIN", headers.Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
	assert.Equal(t, "default-src 'self'; script-src 'self'; object-src 'none';", headers.Get("Content-Security-Policy"))
}

func TestSecureHeaders_DefaultAPI(t *testing.T) {
	router := NewRouter(nil)

	router.Use(SecureHeaders(DefaultAPISecurityConfig()))
	router.Get("/api", func(c *Context) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	headers := rec.Header()
	assert.Equal(t, "0", headers.Get("X-XSS-Protection"))
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", headers.Get("Referrer-Policy"))
	assert.Equal(t, "default-src 'none'; frame-ancestors 'none';", headers.Get("Content-Security-Policy"))
}

func TestSecureHeaders_Overrides(t *testing.T) {
	router := NewRouter(nil)

	customConfig := SecurityConfig{
		FrameOptions: "ALLOW-FROM https://example.com",
	}

	router.Use(SecureHeaders(customConfig))
	router.Get("/", func(c *Context) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// Only overridden fields should be present since we passed an explicit config struct missing the others.
	// SecureHeaders uses the provided struct if given at least one.
	headers := rec.Header()
	assert.Equal(t, "ALLOW-FROM https://example.com", headers.Get("X-Frame-Options"))
	assert.Empty(t, headers.Get("X-XSS-Protection"))
}
