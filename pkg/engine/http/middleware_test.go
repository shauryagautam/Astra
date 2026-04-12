package http

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Just proceed
	})

	handler := RequestID()(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestLogger(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Just proceed
	})
	handler := Logger(slog.Default())(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})
}

func TestRecover(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := Recover(slog.Default())(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})
	assert.Equal(t, 500, w.Code)
}

func TestCORS(t *testing.T) {
	config := CorsConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST"},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Success
	})
	handler := CORS(config)(next)

	t.Run("Preflight", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("Actual Request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestCSRF(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Success
	})
	handler := CSRF(false)(next)

	t.Run("GET generates cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Check for cookie in response
		cookies := w.Result().Cookies()
		found := false
		for _, ck := range cookies {
			if ck.Name == "astra_csrf" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("POST fails without token", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
