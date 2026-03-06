// Package core provides the application bootstrap, service container,
// and lifecycle management for Astra applications.
package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/astraframework/astra/cache"
	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/db"
	"github.com/astraframework/astra/policy"
)

// Service is the interface that all Astra services can optionally implement
// for lifecycle hooks.
type Service interface {
	// Name returns the service name for logging.
	Name() string
}

// Starter is implemented by services that need initialization on app start.
type Starter interface {
	Start(ctx context.Context) error
}

// Stopper is implemented by services that need cleanup on app shutdown.
type Stopper interface {
	Stop(ctx context.Context) error
}

// App is the central application container. It holds configuration, services,
// and manages the application lifecycle.
type App struct {
	Config *config.AstraConfig
	Env    *config.Config
	Logger *slog.Logger
	Gate   *policy.Gate

	mu       sync.RWMutex
	services map[string]any
	onStart  []func(ctx context.Context) error
	onStop   []func(ctx context.Context) error
}

// Option is a functional option for configuring the App.
type Option func(*App)

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) Option {
	return func(a *App) {
		a.Logger = logger
	}
}

// New creates a new App with the given options. It loads the .env file
// and populates the AstraConfig from environment variables.
func New(opts ...Option) (*App, error) {
	envCfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("core.New: %w", err)
	}

	astraCfg := config.LoadFromEnv(envCfg)

	app := &App{
		Config:   astraCfg,
		Env:      envCfg,
		Gate:     policy.New(),
		services: make(map[string]any),
		onStart:  make([]func(ctx context.Context) error, 0),
		onStop:   make([]func(ctx context.Context) error, 0),
	}

	// Default logger
	var handler slog.Handler
	if envCfg.IsProd() {
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

// Register registers a named service in the container.
func (a *App) Register(name string, svc any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.services[name] = svc
}

// Get retrieves a named service from the container. Returns nil if not found.
func (a *App) Get(name string) any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.services[name]
}

// MustGet retrieves a named service or panics if not found.
func (a *App) MustGet(name string) any {
	svc := a.Get(name)
	if svc == nil {
		panic(fmt.Sprintf("core.MustGet: service '%s' not registered", name))
	}
	return svc
}

// OnStart registers a hook to run when the application starts.
func (a *App) OnStart(fn func(ctx context.Context) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onStart = append(a.onStart, fn)
}

// OnStop registers a hook to run when the application stops. Stop hooks
// are executed in reverse order (LIFO).
func (a *App) OnStop(fn func(ctx context.Context) error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onStop = append(a.onStop, fn)
}

// Start boots all registered services and OnStart hooks, then blocks
// until a shutdown signal (SIGTERM/SIGINT) is received.
func (a *App) Start() error {
	ctx := context.Background()

	// 1. Validate config
	if err := a.Config.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	a.Logger.Info("starting application",
		"name", a.Config.App.Name,
		"env", a.Config.App.Environment,
	)

	// 2. Auto-connect Postgres
	if a.Get("db") == nil && a.Config.Database.URL != "" {
		orm, pool, err := db.Connect(context.Background(), a.Config.Database)
		if err != nil {
			return err
		}
		dbSvc := db.New(a.Config.Database)
		dbSvc.Orm = orm
		dbSvc.Pool = pool
		a.Register("db", dbSvc)
	}

	// 3. Auto-connect Redis / Cache
	if a.Get("cache") == nil {
		cacheStore, err := a.connectCache()
		if err != nil {
			return err
		}
		a.Register("cache", cacheStore)
	}

	// Run OnStart hooks
	a.mu.RLock()
	startHooks := make([]func(ctx context.Context) error, len(a.onStart))
	copy(startHooks, a.onStart)
	a.mu.RUnlock()

	for _, fn := range startHooks {
		if err := fn(ctx); err != nil {
			return fmt.Errorf("core.Start: start hook failed: %w", err)
		}
	}

	// Start services that implement Starter
	a.mu.RLock()
	for name, svc := range a.services {
		if starter, ok := svc.(Starter); ok {
			a.Logger.Info("starting service", "service", name)
			if err := starter.Start(ctx); err != nil {
				a.mu.RUnlock()
				return fmt.Errorf("core.Start: service '%s' failed to start: %w", name, err)
			}
		}
	}
	a.mu.RUnlock()

	a.Logger.Info("application started successfully")

	// Block until shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	a.Logger.Info("shutdown signal received", "signal", sig.String())
	return a.Shutdown()
}

func (a *App) connectCache() (any, error) {
	// Try Redis if URL is provided
	if a.Config.Redis.URL != "" || a.Config.Redis.Host != "" {
		client, err := cache.ConnectRedis(a.Config.Redis)
		if err == nil {
			return cache.NewStore(client), nil
		}
		a.Logger.Warn("Redis connection failed, falling back to memory cache", "error", err)
	}

	// Fallback to memory cache
	a.Logger.Info("using in-memory cache")
	return cache.NewMemoryStore(), nil
}

// Shutdown gracefully shuts down the application. It runs all OnStop hooks
// and stops services in reverse order with a 15-second timeout.
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	a.Logger.Info("shutting down gracefully...")

	// Run OnStop hooks in reverse order
	a.mu.RLock()
	stopHooks := make([]func(ctx context.Context) error, len(a.onStop))
	copy(stopHooks, a.onStop)
	a.mu.RUnlock()

	var firstErr error
	for i := len(stopHooks) - 1; i >= 0; i-- {
		if err := stopHooks[i](ctx); err != nil {
			a.Logger.Error("stop hook failed", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// Stop services that implement Stopper
	a.mu.RLock()
	for name, svc := range a.services {
		if stopper, ok := svc.(Stopper); ok {
			a.Logger.Info("stopping service", "service", name)
			if err := stopper.Stop(ctx); err != nil {
				a.Logger.Error("service stop failed", "service", name, "error", err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	a.mu.RUnlock()

	a.Logger.Info("application stopped")
	return firstErr
}
