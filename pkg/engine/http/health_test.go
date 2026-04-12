package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadyHandler(t *testing.T) {
	app := NewTestApp()
	router := NewRouter(app.Config(), app.Logger())
	router.Get("/ready", ReadyHandler())

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestHealthHandler_OK(t *testing.T) {
	app := NewTestApp()
	app.RegisterHealthCheck("db", engine.HealthCheckFunc(func(ctx context.Context) error {
		return nil
	}))

	router := NewRouter(app.Config(), app.Logger())
	router.Get("/health", HealthHandler(app.GetHealthChecks()))

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.Contains(t, rec.Body.String(), `"db"`)
}

func TestHealthHandler_Error(t *testing.T) {
	app := NewTestApp()
	app.RegisterHealthCheck("redis", engine.HealthCheckFunc(func(ctx context.Context) error {
		return errors.New("connection refused")
	}))

	router := NewRouter(app.Config(), app.Logger())
	router.Get("/health", HealthHandler(app.GetHealthChecks()))

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"error"`)
	assert.Contains(t, rec.Body.String(), `"connection refused"`)
}
