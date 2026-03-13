package telemetry

import (
	"context"

	"github.com/astraframework/astra/core"
)

// TelemetryProvider implements core.Provider for telemetry services.
type TelemetryProvider struct {
	core.BaseProvider
	checker *HealthChecker
}

// NewTelemetryProvider creates a new TelemetryProvider.
func NewTelemetryProvider() *TelemetryProvider {
	return &TelemetryProvider{
		checker: NewHealthChecker(),
	}
}

// Register registers the HealthChecker in the container.
func (p *TelemetryProvider) Register(app *core.App) error {
	app.Register("health", p.checker)
	return nil
}

// Boot initializes health checks and OpenTelemetry tracer.
func (p *TelemetryProvider) Boot(app *core.App) error {
	// 1. Initialize OpenTelemetry
	if app.Config.Telemetry.Endpoint != "" {
		shutdown, err := InitTracer(context.Background(), app.Config.Telemetry.Endpoint, app.Config.Telemetry.ServiceName)
		if err != nil {
			return err
		}
		app.OnStop(func(ctx context.Context) error {
			return shutdown(ctx)
		})
	}

	// 2. Register health checks
	checks := app.GetHealthChecks()
	for name, check := range checks {
		if fn, ok := check.(func(context.Context) error); ok {
			p.checker.Register(name, fn)
		} else if fn, ok := check.(HealthCheckFunc); ok {
			p.checker.Register(name, fn)
		}
	}
	return nil
}

// Shutdown gracefully stops any telemetry background tasks.
func (p *TelemetryProvider) Shutdown(ctx context.Context, app *core.App) error {
	return nil
}
