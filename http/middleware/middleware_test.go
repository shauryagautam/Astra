package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	astrahttp "github.com/astraframework/astra/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID(t *testing.T) {
	next := func(c *astrahttp.Context) error {
		_ = c.Request.Header.Get("X-Request-ID")
		return nil
	}

	handler := RequestID()(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	c := astrahttp.NewContext(w, req, nil)

	err := handler(c)
	require.NoError(t, err)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestLogger(t *testing.T) {
	next := func(c *astrahttp.Context) error {
		return nil
	}
	handler := Logger(slog.Default())(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	c := astrahttp.NewContext(w, req, nil)

	err := handler(c)
	require.NoError(t, err)
}

func TestRecover(t *testing.T) {
	next := func(c *astrahttp.Context) error {
		panic("test panic")
	}
	handler := Recover(slog.Default())(next)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	c := astrahttp.NewContext(w, req, nil)

	var err error
	assert.NotPanics(t, func() {
		err = handler(c)
	})
	assert.Error(t, err)
	// Check if it's an HTTPError with 500
	if httpErr, ok := err.(*astrahttp.HTTPError); ok {
		assert.Equal(t, 500, httpErr.Status)
	} else {
		t.Errorf("expected *astrahttp.HTTPError, got %T", err)
	}
}

func TestCORS(t *testing.T) {
	config := CorsConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST"},
	}

	next := func(c *astrahttp.Context) error {
		return nil
	}
	handler := CORS(config)(next)

	t.Run("Preflight", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("Actual Request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		require.NoError(t, err)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestCSRF(t *testing.T) {
	next := func(c *astrahttp.Context) error {
		return nil
	}
	handler := CSRF()(next)

	t.Run("GET generates cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		require.NoError(t, err)

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
		c := astrahttp.NewContext(w, req, nil)

		err := handler(c)
		assert.Error(t, err)
		// CSRF error usually returns an error that router handles,
		// but since we call handler(c) directly, it returns the error.
	})
}
