package engine

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/shauryagautam/Astra/pkg/engine/config"
)

// App is the pure Lifecycle Manager of the Astra framework.
// It manages the application context, startup/shutdown hooks, and providers.
// It no longer acts as a service locator; services are explicitly injected into components via Wire.
type App struct {
	mu        sync.RWMutex
	config    *config.AstraConfig
	env       *config.Config
	logger    *slog.Logger

	providers []Provider
	ctx       context.Context
	cancel    context.CancelFunc

	onStart []func(context.Context) error
	onStop  []func(context.Context) error

	healthChecks map[string]HealthProvider
}

// New creates a new Astra application kernel with minimal core dependencies.
func New(
	config *config.AstraConfig,
	env *config.Config,
	logger *slog.Logger,
) *App {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return &App{
		config:       config,
		env:          env,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		providers:    make([]Provider, 0),
		onStart:      make([]func(context.Context) error, 0),
		onStop:       make([]func(context.Context) error, 0),
		healthChecks: make(map[string]HealthProvider),
	}
}

// Config returns the application configuration.
func (a *App) Config() *config.AstraConfig { return a.config }

// Env returns the environment variables.
func (a *App) Env() *config.Config { return a.env }

// Logger returns the application logger.
func (a *App) Logger() *slog.Logger { return a.logger }

// BaseContext returns the application's base context.
func (a *App) BaseContext() context.Context { return a.ctx }

// OnStart registers a hook to run when the app boots.
// This method is thread-safe and wraps hooks with context protection during execution.
func (a *App) OnStart(fn func(context.Context) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onStart = append(a.onStart, fn)
}

// OnStop registers a shutdown hook.
// This method is thread-safe and hooks are executed in reverse order during shutdown.
func (a *App) OnStop(fn func(context.Context) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onStop = append(a.onStop, fn)
}

// Run boots the application and blocks until a termination signal is received.
// It handles the full lifecycle from Boot to Graceful Shutdown.
func (a *App) Run() error {
	if err := a.Boot(); err != nil {
		return err
	}
	
	a.logger.Info("Astra kernel is running. Press Ctrl+C to stop.")
	<-a.BaseContext().Done()
	
	a.logger.Info("Shutdown signal received. Cleaning up...")
	return a.Shutdown()
}

// Shutdown gracefully stops the application.
// It executes onStop hooks and provider shutdown methods in reverse order of registration.
// Aggregates all errors encountered using errors.Join for a single cohesive return.
// It uses a fresh 15-second timeout context to guarantee termination.
func (a *App) Shutdown() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cancel()

	// Hardened Shutdown Protection: fresh context to ensure cleanup completes even if base ctx is canceled
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var errs []error

	// Execute onStop hooks in reverse order (LIFO)
	for i := len(a.onStop) - 1; i >= 0; i-- {
		if err := a.onStop[i](ctx); err != nil {
			a.logger.Error("onStop hook failed", "error", err)
			errs = append(errs, err)
		}
	}

	// Shutdown providers in reverse order of registration
	for i := len(a.providers) - 1; i >= 0; i-- {
		p := a.providers[i]
		if err := p.Shutdown(ctx, a); err != nil {
			a.logger.Error("provider shutdown failed", "name", p.Name(), "error", err)
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Recover handles application panics by logging the error.
func (a *App) Recover() {
	if r := recover(); r != nil {
		a.logger.Error("app panic recovered", "error", r)
	}
}

// GetHealthChecks returns all registered health providers.
// This method is thread-safe and returns a point-in-time snapshot of health checks.
func (a *App) GetHealthChecks() map[string]HealthProvider {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	checks := make(map[string]HealthProvider, len(a.healthChecks))
	for k, v := range a.healthChecks {
		checks[k] = v
	}
	return checks
}

// RegisterHealthCheck registers a new health check provider.
// This method is thread-safe.
func (a *App) RegisterHealthCheck(name string, check HealthProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.healthChecks == nil {
		a.healthChecks = make(map[string]HealthProvider)
	}
	a.healthChecks[name] = check
}

// RegisterProvider adds a provider to the application.
// This method is thread-safe.
func (a *App) RegisterProvider(p Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers = append(a.providers, p)
}

// Boot initializes all registered providers in a strict three-phase sequence:
// Register → Boot → Ready. The Ready phase only executes once all providers
// have completed their Boot phase. OnStart hooks are wrapped in a 30s timeout.
func (a *App) Boot() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Phase 1: Register - All providers define their presence
	for _, p := range a.providers {
		if err := p.Register(a); err != nil {
			return err
		}
	}

	// Phase 2: Boot - All providers perform initialization
	for _, p := range a.providers {
		if err := p.Boot(a); err != nil {
			return err
		}
	}

	// Phase 3: Ready - All providers confirm operational readiness
	for _, p := range a.providers {
		if err := p.Ready(a); err != nil {
			return err
		}
	}

	// Startup Protection: Wrap OnStart hooks with a 30-second context timeout
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	for _, fn := range a.onStart {
		if err := fn(ctx); err != nil {
			return err
		}
	}

	return nil
}
