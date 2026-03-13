package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraframework/astra/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadyHandler(t *testing.T) {
	app, _ := core.New()
	router := NewRouter(app)
	router.Get("/ready", ReadyHandler())

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestHealthHandler_OK(t *testing.T) {
	app, _ := core.New()
	app.RegisterHealthCheck("db", func(ctx context.Context) error {
		return nil
	})

	router := NewRouter(app)
	router.Get("/health", HealthHandler())

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.Contains(t, rec.Body.String(), `"db"`)
}

func TestHealthHandler_Error(t *testing.T) {
	app, _ := core.New()
	app.RegisterHealthCheck("redis", func(ctx context.Context) error {
		return errors.New("connection refused")
	})

	router := NewRouter(app)
	router.Get("/health", HealthHandler())

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"error"`)
	assert.Contains(t, rec.Body.String(), `"connection refused"`)
}
