package core

// Astra core package.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/container"
)

// App is the central application container. It holds configuration, services,
// and manages the application lifecycle.
type App struct {
	Config    *config.AstraConfig
	Env       *config.Config
	Container *container.Container
	Logger    *slog.Logger
	Gate      interface {
		Allows(user any, action string, subject any) bool
	}

	services  map[string]any
	mu        sync.RWMutex
	onStart   []func(ctx context.Context) error
	onStop    []func(ctx context.Context) error
	providers []Provider

	booted       bool
	healthChecks map[string]any
}

// Option is a functional option for configuring the App.
type Option func(*App)

// New creates a new App with the given options.
func New(opts ...Option) (*App, error) {
	env, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("core.New: failed to load config: %w", err)
	}

	app := &App{
		Env:          env,
		Config:       config.LoadFromEnv(env),
		Container:    container.New(),
		onStart:      make([]func(ctx context.Context) error, 0),
		onStop:       make([]func(ctx context.Context) error, 0),
		services:     make(map[string]any),
		healthChecks: make(map[string]any),
	}

	// Default Logger
	var handler slog.Handler
	if env.IsProd() {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	app.Logger = slog.New(handler)

	for _, opt := range opts {
		opt(app)
	}

	return app, nil
}

// OnStart registers a hook to run when the application starts.
func (a *App) OnStart(fn func(ctx context.Context) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onStart = append(a.onStart, fn)
}

// OnStop registers a hook to run when the application stops.
func (a *App) OnStop(fn func(ctx context.Context) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onStop = append(a.onStop, fn)
}

// Start boots the application and blocks until a signal is received.
func (a *App) Start() error {
	defer a.Recover()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := a.Boot(ctx); err != nil {
		return err
	}

	// Wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	a.Logger.Info("shutting down...", "signal", sig.String())

	return a.Shutdown(a.Config.App.ShutdownTimeout)
}

// Boot executes all Provider methods and OnStart hooks in order.
func (a *App) Boot(ctx context.Context) error {
	defer a.Recover()
	a.mu.Lock()
	if a.booted {
		a.mu.Unlock()
		return nil
	}
	providers := append([]Provider{}, a.providers...)
	hooks := append([]func(context.Context) error{}, a.onStart...)
	a.mu.Unlock()

	start := time.Now()

	// 1. Register phase
	for _, p := range providers {
		if err := p.Register(a); err != nil {
			return fmt.Errorf("core: provider register failed: %w", err)
		}
	}

	// 2. Boot phase
	for _, p := range providers {
		if err := p.Boot(a); err != nil {
			return fmt.Errorf("core: provider boot failed: %w", err)
		}
	}

	// 3. Start hooks
	for _, hook := range hooks {
		if err := hook(ctx); err != nil {
			return fmt.Errorf("core: start hook failed: %w", err)
		}
	}

	// 4. Ready phase
	for _, p := range providers {
		if err := p.Ready(a); err != nil {
			return fmt.Errorf("core: provider ready failed: %w", err)
		}
	}

	a.mu.Lock()
	a.booted = true
	a.mu.Unlock()

	a.Logger.Info("application started", "duration", time.Since(start).String())
	return nil
}

// Shutdown gracefully shuts down the application by running OnStop hooks and Provider Shutdown in reverse.
func (a *App) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	a.mu.Lock()
	providers := append([]Provider{}, a.providers...)
	hooks := append([]func(context.Context) error{}, a.onStop...)
	a.mu.Unlock()

	var firstErr error
	// 1. Cleanup hooks (reverse order)
	for i := len(hooks) - 1; i >= 0; i-- {
		// Respect context timeout
		if ctx.Err() != nil {
			a.Logger.Error("shutdown timed out during hooks")
			return ctx.Err()
		}
		if err := hooks[i](ctx); err != nil {
			a.Logger.Error("shutdown hook failed", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// 2. Shutdown providers (reverse order)
	for i := len(providers) - 1; i >= 0; i-- {
		// Respect context timeout
		if ctx.Err() != nil {
			a.Logger.Error("shutdown timed out during provider shutdown")
			return ctx.Err()
		}
		if err := providers[i].Shutdown(ctx, a); err != nil {
			a.Logger.Error("provider shutdown failed", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	a.Logger.Info("graceful shutdown complete")
	return firstErr
}

// Recover handles panics, logs them with stack trace, and prevents application crash.
func (a *App) Recover() {
	if r := recover(); r != nil {
		a.Logger.Error("panic recovered",
			"panic", r,
			"stack", string(debug.Stack()),
		)
	}
}

// Get retrieves a service by name.
func (a *App) Get(name string) any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.services[name]
}

// Has checks if a service is registered in the application.
func (a *App) Has(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.services[name]
	return ok
}

// Register adds a service to the application.
func (a *App) Register(name string, service any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.services[name] = service
}

// Use appends a provider to the application and registers it.
func (a *App) Use(p Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers = append(a.providers, p)
}

// Providers returns all registered providers.
func (a *App) Providers() []Provider {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.providers
}

// RegisterHealthCheck registers a health check for a component.
func (a *App) RegisterHealthCheck(name string, check any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.healthChecks[name] = check
}

// GetHealthChecks returns all registered health checks.
func (a *App) GetHealthChecks() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make(map[string]any)
	for k, v := range a.healthChecks {
		out[k] = v
	}
	return out
}
