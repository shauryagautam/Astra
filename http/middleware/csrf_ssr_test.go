package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	astrahttp "github.com/astraframework/astra/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSRF_SSR(t *testing.T) {
	next := func(c *astrahttp.Context) error {
		return c.SendString("ok")
	}
	handler := CSRF()(next)

	t.Run("POST with _csrf form field", func(t *testing.T) {
		token := "test-token"
		form := url.Values{}
		form.Add("_csrf", token)

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: token})

		w := httptest.NewRecorder()
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("POST with astra_csrf form field", func(t *testing.T) {
		token := "test-token"
		form := url.Values{}
		form.Add("astra_csrf", token)

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: token})

		w := httptest.NewRecorder()
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("POST with header still works", func(t *testing.T) {
		token := "test-token"
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("X-CSRF-Token", token)
		req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: token})

		w := httptest.NewRecorder()
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("POST fails with mismatched token", func(t *testing.T) {
		form := url.Values{}
		form.Add("_csrf", "wrong-token")

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: "valid-token"})

		w := httptest.NewRecorder()
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		assert.Error(t, err)
		if httpErr, ok := err.(*astrahttp.HTTPError); ok {
			assert.Equal(t, http.StatusForbidden, httpErr.Status)
		}
	})
}
