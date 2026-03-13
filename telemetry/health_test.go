package telemetry

import (
	"context"
	"errors"
	"testing"

	"github.com/astraframework/astra/core"
	"github.com/stretchr/testify/assert"
)

func TestHealthChecker(t *testing.T) {
	checker := NewHealthChecker()

	checker.Register("test_ok", func(ctx context.Context) error {
		return nil
	})

	checker.Register("test_err", func(ctx context.Context) error {
		return errors.New("boom")
	})

	reportObj := checker.Report(context.Background(), "L1")
	report := reportObj.(HealthReport)

	assert.Equal(t, StatusError, report.Status)
	assert.Equal(t, StatusOk, report.Components["test_ok"].Status)
	assert.Equal(t, StatusError, report.Components["test_err"].Status)
	assert.Equal(t, "boom", report.Components["test_err"].Message)
}

func TestTelemetryProvider(t *testing.T) {
	app, _ := core.New()

	// Register a health check in the app
	app.RegisterHealthCheck("db", func(ctx context.Context) error {
		return nil
	})

	provider := NewTelemetryProvider()
	app.Use(provider)

	// Simulate Boot phase
	err := provider.Register(app)
	assert.NoError(t, err)

	err = provider.Boot(app)
	assert.NoError(t, err)

	// Retrieve health service
	svc := app.Get("health")
	assert.NotNil(t, svc)

	checker := svc.(*HealthChecker)
	reportObj := checker.Report(context.Background(), "L1")
	report := reportObj.(HealthReport)

	assert.Equal(t, StatusOk, report.Status)
	assert.Contains(t, report.Components, "db")
	assert.Equal(t, StatusOk, report.Components["db"].Status)
}
