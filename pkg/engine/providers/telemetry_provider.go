package providers

import (
	"context"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/observability/health"
	"github.com/shauryagautam/Astra/pkg/observability/metrics"
	"github.com/shauryagautam/Astra/pkg/observability/trace"
)

// ObservabilityProvider implements engine.Provider for observability services.
type ObservabilityProvider struct {
	engine.BaseProvider
	checker *health.HealthChecker
}

// NewObservabilityProvider creates a new ObservabilityProvider.
func NewObservabilityProvider() *ObservabilityProvider {
	return &ObservabilityProvider{
		checker: health.NewHealthChecker(),
	}
}

// Register is a no-op for ObservabilityProvider.
func (p *ObservabilityProvider) Register(app *engine.App) error {
	return nil
}

// Boot initializes health checks, tracing, and metrics.
func (p *ObservabilityProvider) Boot(app *engine.App) error {
	// 1. Initialize Tracing
	if app.Config().Telemetry.Endpoint != "" {
		shutdown, err := trace.Init(context.Background(), app.Config().Telemetry.Endpoint, app.Config().Telemetry.ServiceName)
		if err != nil {
			return err
		}
		app.OnStop(func(ctx context.Context) error {
			return shutdown(ctx)
		})
	}

	// 2. Initialize Metrics
	if app.Config().Telemetry.Endpoint != "" {
		shutdown, err := metrics.Init(context.Background(), app.Config().Telemetry.Endpoint, app.Config().Telemetry.ServiceName)
		if err != nil {
			return err
		}
		app.OnStop(func(ctx context.Context) error {
			return shutdown(ctx)
		})
	}

	// 3. Register health checks
	checks := app.GetHealthChecks()
	for name, check := range checks {
		chk := check // capture
		p.checker.Register(name, func(ctx context.Context) error {
			return chk.CheckHealth(ctx)
		})
	}
	return nil
}

// Shutdown gracefully stops any observability background tasks.
func (p *ObservabilityProvider) Shutdown(ctx context.Context, app *engine.App) error {
	return nil
}
