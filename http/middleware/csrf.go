package middleware

import (
	"crypto/rand"
	"crypto/subtle"
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
					if err := setCSRFCookie(c); err != nil {
						return err
					}
				}
				return next(c)
			}

			// Verify token for unsafe methods
			cookie, err := c.Request.Cookie(csrfCookieName)
			if err != nil {
				return astrahttp.NewHTTPError(http.StatusForbidden, "CSRF_TOKEN_MISSING", "CSRF token cookie is missing")
			}

			headerToken := c.Request.Header.Get(csrfHeaderName)
			formToken := c.Request.FormValue("_csrf")
			if formToken == "" {
				formToken = c.Request.FormValue("astra_csrf")
			}

			token := headerToken
			if token == "" {
				token = formToken
			}

			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
				return astrahttp.NewHTTPError(http.StatusForbidden, "CSRF_TOKEN_INVALID", "CSRF token is invalid or missing")
			}

			return next(c)
		}
	}
}

func setCSRFCookie(c *astrahttp.Context) error {
	token, err := generateRandomToken(32)
	if err != nil {
		return err
	}
	secure := c.App != nil && c.App.Env.IsProd()
	c.SetCookie(&http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	return nil
}

func generateRandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
