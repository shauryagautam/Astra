package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	astrahttp "github.com/astraframework/astra/http"
)

const (
	csrfCookieName = "astra_csrf"
	csrfHeaderName = "X-CSRF-Token"
)

// CSRF implements Cross-Site Request Forgery protection.
// It generates a token, stores it in an HTTP-only cookie, and verifies
// that subsequent non-GET requests include a matching token in the header.
func CSRF() astrahttp.MiddlewareFunc {
	return func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
		return func(c *astrahttp.Context) error {
			// Skip for GET, HEAD, OPTIONS (safe methods)
			if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
				// Ensure cookie exists even for safe methods
				if _, err := c.Request.Cookie(csrfCookieName); err != nil {
					setCSRFCookie(c)
				}
				return next(c)
			}

			// Verify token for unsafe methods
			cookie, err := c.Request.Cookie(csrfCookieName)
			if err != nil {
				return astrahttp.NewHTTPError(http.StatusForbidden, "CSRF_TOKEN_MISSING", "CSRF token cookie is missing")
			}

			headerToken := c.Request.Header.Get(csrfHeaderName)
			if headerToken == "" || headerToken != cookie.Value {
				return astrahttp.NewHTTPError(http.StatusForbidden, "CSRF_TOKEN_INVALID", "CSRF token is invalid or missing from header")
			}

			return next(c)
		}
	}
}

func setCSRFCookie(c *astrahttp.Context) {
	token := generateRandomToken(32)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})
}

func generateRandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}
