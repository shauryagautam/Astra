package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/shauryagautam/Astra/pkg/engine/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

	// parentCtx holds the externally-supplied system context (e.g. from Kubernetes
	// orchestration). Boot and Shutdown timeout derivations are chained off this
	// context so that upstream cancellations propagate immediately instead of
	// blocking for the full fallback timeout.
	parentCtx context.Context

	onStart []func(context.Context) error
	onStop  []func(context.Context) error

	healthChecks map[string]HealthProvider
}

// New creates a new Astra application kernel with minimal core dependencies.
// The signal-notification context is established here; callers that need to
// supply a custom parent context should use NewWithContext instead.
func New(
	config *config.AstraConfig,
	env *config.Config,
	logger *slog.Logger,
) *App {
	return NewWithContext(context.Background(), config, env, logger)
}

// NewWithContext creates a new Astra application kernel derived from the
// supplied parent context. This is the canonical entry-point when running
// inside an orchestration layer (Kubernetes, systemd) that provides its own
// cancellation chain. The signal-notification context wraps parentCtx so that
// both SIGTERM signals and upstream context cancellations cause a clean exit.
func NewWithContext(
	parentCtx context.Context,
	cfg *config.AstraConfig,
	env *config.Config,
	logger *slog.Logger,
) *App {
	signalCtx, cancel := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	return &App{
		config:       cfg,
		env:          env,
		logger:       logger,
		ctx:          signalCtx,
		cancel:       cancel,
		parentCtx:    parentCtx,
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

// RouterProvider is a sub-interface implemented by providers that expose the router.
type RouterProvider interface {
	GetRouter() any
}

// Router returns the HTTP router if one of the providers implements RouterProvider.
func (a *App) Router() any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, p := range a.providers {
		if rp, ok := p.(RouterProvider); ok {
			return rp.GetRouter()
		}
	}
	return nil
}

// Start boots the application and blocks until a termination signal is received.
// It is an alias for Run() to support scaffolding templates.
func (a *App) Start() error {
	return a.Run()
}

// Run boots the application and blocks until a termination signal is received.
// It handles the full lifecycle from Boot to Graceful Shutdown.
// The internal parent context (set by New or NewWithContext) is used for all
// timeout derivations throughout the lifecycle.
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
//
// The 15-second termination budget is now derived from the parent system context
// (set at construction time) rather than context.Background(). If the orchestration
// layer cancels the parent context before the 15-second window expires, the
// shutdown loop will respect that cancellation immediately.
func (a *App) Shutdown() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cancel()

	// Derive the shutdown budget from the parent context so that orchestration
	// signals (e.g. a Kubernetes SIGTERM deadline) can interrupt the cleanup
	// loop before the full 15-second fallback expires.
	ctx, cancel := context.WithTimeout(a.parentCtx, 15*time.Second)
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

// Recover handles application panics with full structured context.
//
// It captures the raw panic value, collects a runtime stack trace (up to 8 KiB)
// and, when an active OpenTelemetry span is available on the application's base
// context, records the panic as a span event and marks the span as errored before
// logging the fatal entry.  This ensures distributed traces surface panics with
// full attribution rather than simply disappearing.
//
// Usage – defer this at the top of any goroutine that must survive panics:
//
//	defer app.Recover()
func (a *App) Recover() {
	r := recover()
	if r == nil {
		return
	}

	// Collect the full goroutine stack trace for deep diagnostics.
	const maxStackSize = 8 << 10 // 8 KiB
	stackBuf := make([]byte, maxStackSize)
	n := runtime.Stack(stackBuf, false)
	stackTrace := string(stackBuf[:n])

	panicMsg := fmt.Sprintf("%v", r)

	// Inject the panic event and stack trace into the active OTel span (if any).
	// We read from the kernel's base context which is the canonical carrier for
	// span propagation across Astra components.
	if span := trace.SpanFromContext(a.ctx); span != nil && span.IsRecording() {
		span.SetStatus(codes.Error, "unhandled panic recovered")
		span.RecordError(
			fmt.Errorf("panic: %s", panicMsg),
			trace.WithStackTrace(true),
		)
		span.SetAttributes(
			attribute.String("panic.value", panicMsg),
			attribute.String("panic.stack_trace", stackTrace),
		)
	}

	a.logger.Error("app panic recovered",
		"panic_value", panicMsg,
		"stack_trace", stackTrace,
	)
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
// have completed their Boot phase.
//
// OnStart hooks run inside a 30-second window derived from the parent system
// context so that orchestration-layer cancellations are respected during boot.
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

	// Startup Protection: Derive the 30-second budget from the parent context so
	// that an upstream cancellation (e.g. a Kubernetes pre-stop hook) immediately
	// aborts the boot sequence rather than blocking for the full window.
	ctx, cancel := context.WithTimeout(a.parentCtx, 30*time.Second)
	defer cancel()

	for _, fn := range a.onStart {
		if err := fn(ctx); err != nil {
			return err
		}
	}

	return nil
}
