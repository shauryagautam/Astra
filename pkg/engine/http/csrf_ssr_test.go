package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSRF_SSR(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	handler := CSRF(false)(next)

	t.Run("POST with _csrf form field", func(t *testing.T) {
		token := "test-token"
		form := url.Values{}
		form.Add("_csrf", token)

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: token})

		w := httptest.NewRecorder()
		
		c := NewContext(w, req)
		ctx := context.WithValue(req.Context(), astraContextKey, c)
		req = req.WithContext(ctx)

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
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
		
		c := NewContext(w, req)
		ctx := context.WithValue(req.Context(), astraContextKey, c)
		req = req.WithContext(ctx)

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("POST with header still works", func(t *testing.T) {
		token := "test-token"
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("X-CSRF-Token", token)
		req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: token})

		w := httptest.NewRecorder()
		
		c := NewContext(w, req)
		ctx := context.WithValue(req.Context(), astraContextKey, c)
		req = req.WithContext(ctx)

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("POST fails with mismatched token", func(t *testing.T) {
		form := url.Values{}
		form.Add("_csrf", "wrong-token")

		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "astra_csrf", Value: "valid-token"})

		w := httptest.NewRecorder()
		
		c := NewContext(w, req)
		ctx := context.WithValue(req.Context(), astraContextKey, c)
		req = req.WithContext(ctx)

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
