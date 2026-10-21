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

const (
	startTimeout = 30 * time.Second
	stopTimeout  = 15 * time.Second
)

type lifecycleState uint8

const (
	stateNew lifecycleState = iota
	stateStarting
	stateRunning
	stateStopping
	stateStopped
)

// App is Astra's explicit lifecycle manager. Components are supplied by
// constructors or Wire and are started in registration order and stopped in
// reverse order. App does not resolve or own application services.
type App struct {
	mu     sync.RWMutex
	state  lifecycleState
	config *config.AstraConfig
	env    *config.Config
	logger *slog.Logger

	components []Component
	ctx        context.Context
	cancel     context.CancelFunc
	parentCtx  context.Context

	onStart []func(context.Context) error
	onStop  []func(context.Context) error

	healthChecks map[string]HealthCheck

	stopDone chan struct{}
	stopErr  error
}

// New creates a kernel using a background parent context.
func New(cfg *config.AstraConfig, env *config.Config, logger *slog.Logger) *App {
	return NewWithContext(context.Background(), cfg, env, logger)
}

// NewWithContext creates a kernel whose lifecycle is bounded by parentCtx.
// This is the entry point for applications managed by Kubernetes or another
// external supervisor.
func NewWithContext(parentCtx context.Context, cfg *config.AstraConfig, env *config.Config, logger *slog.Logger) *App {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	return &App{
		config:       cfg,
		env:          env,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		parentCtx:    parentCtx,
		healthChecks: make(map[string]HealthCheck),
		stopDone:     make(chan struct{}),
	}
}

func (a *App) Config() *config.AstraConfig  { return a.config }
func (a *App) Env() *config.Config          { return a.env }
func (a *App) Logger() *slog.Logger         { return a.logger }
func (a *App) BaseContext() context.Context { return a.ctx }

// RegisterComponent registers a lifecycle component. Components must be
// registered before Boot and are started in this order.
func (a *App) RegisterComponent(c Component) error {
	if c == nil {
		return errors.New("engine: cannot register a nil component")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state != stateNew {
		return errors.New("engine: components must be registered before Boot")
	}
	a.components = append(a.components, c)
	return nil
}

// OnStart adds a startup hook. Hooks are snapshotted before execution and may
// be registered concurrently before startup begins.
func (a *App) OnStart(fn func(context.Context) error) {
	if fn == nil {
		return
	}
	a.mu.Lock()
	a.onStart = append(a.onStart, fn)
	a.mu.Unlock()
}

// OnStop adds a shutdown hook. Hooks are run in reverse registration order.
func (a *App) OnStop(fn func(context.Context) error) {
	if fn == nil {
		return
	}
	a.mu.Lock()
	a.onStop = append(a.onStop, fn)
	a.mu.Unlock()
}

// Start is an alias for Run.
func (a *App) Start() error { return a.Run() }

// Run starts the application and waits for its lifecycle context to finish.
func (a *App) Run() error {
	if err := a.Boot(); err != nil {
		return err
	}
	<-a.BaseContext().Done()
	a.logger.Info("Shutdown signal received. Cleaning up...")
	return a.Shutdown()
}

// Boot starts all components in dependency order, followed by startup hooks.
// A component contract is Start(ctx) error and Stop(ctx) error; Start must
// honor ctx and must not return until the component is ready or has failed.
func (a *App) Boot() error {
	a.mu.Lock()
	if a.state != stateNew {
		a.mu.Unlock()
		return errors.New("engine: application has already started")
	}
	a.state = stateStarting
	components := append([]Component(nil), a.components...)
	startHooks := append([]func(context.Context) error(nil), a.onStart...)
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(a.parentCtx, startTimeout)
	defer cancel()
	for _, component := range components {
		if err := ctx.Err(); err != nil {
			a.markStartFailed()
			return err
		}
		if err := component.Start(ctx); err != nil {
			a.markStartFailed()
			return fmt.Errorf("engine: start component: %w", err)
		}
	}
	for _, hook := range startHooks {
		if err := ctx.Err(); err != nil {
			a.markStartFailed()
			return err
		}
		if err := hook(ctx); err != nil {
			a.markStartFailed()
			return fmt.Errorf("engine: start hook: %w", err)
		}
	}

	a.mu.Lock()
	a.state = stateRunning
	a.mu.Unlock()
	return nil
}

func (a *App) markStartFailed() {
	a.mu.Lock()
	a.state = stateStopped
	a.mu.Unlock()
}

// Shutdown stops hooks and components in reverse order. It is safe to call
// concurrently; all callers observe the same result.
func (a *App) Shutdown() error {
	a.mu.Lock()
	switch a.state {
	case stateStopping:
		done := a.stopDone
		a.mu.Unlock()
		<-done
		return a.stopErr
	case stateStopped:
		err := a.stopErr
		a.mu.Unlock()
		return err
	default:
		a.state = stateStopping
		components := append([]Component(nil), a.components...)
		stopHooks := append([]func(context.Context) error(nil), a.onStop...)
		a.cancel()
		a.mu.Unlock()

		ctx, cancel := context.WithTimeout(a.parentCtx, stopTimeout)
		defer cancel()
		var errs []error
		for i := len(stopHooks) - 1; i >= 0; i-- {
			if err := stopHooks[i](ctx); err != nil {
				a.logger.Error("engine: stop hook failed", "error", err)
				errs = append(errs, err)
			}
		}
		for i := len(components) - 1; i >= 0; i-- {
			if err := components[i].Stop(ctx); err != nil {
				a.logger.Error("engine: stop component failed", "error", err)
				errs = append(errs, err)
			}
		}
		err := errors.Join(errs...)
		a.mu.Lock()
		a.stopErr = err
		a.state = stateStopped
		close(a.stopDone)
		a.mu.Unlock()
		return err
	}
}

func (a *App) RegisterHealthCheck(name string, check HealthCheck) {
	a.mu.Lock()
	a.healthChecks[name] = check
	a.mu.Unlock()
}

func (a *App) GetHealthChecks() map[string]HealthCheck {
	a.mu.RLock()
	defer a.mu.RUnlock()
	checks := make(map[string]HealthCheck, len(a.healthChecks))
	for name, check := range a.healthChecks {
		checks[name] = check
	}
	return checks
}

func (a *App) Recover() {
	r := recover()
	if r == nil {
		return
	}
	const maxStackSize = 8 << 10
	stackBuf := make([]byte, maxStackSize)
	n := runtime.Stack(stackBuf, false)
	stackTrace := string(stackBuf[:n])
	panicMsg := fmt.Sprintf("%v", r)
	if span := trace.SpanFromContext(a.ctx); span != nil && span.IsRecording() {
		span.SetStatus(codes.Error, "unhandled panic recovered")
		span.RecordError(fmt.Errorf("panic: %s", panicMsg), trace.WithStackTrace(true))
		span.SetAttributes(attribute.String("panic.value", panicMsg), attribute.String("panic.stack_trace", stackTrace))
	}
	a.logger.Error("app panic recovered", "panic_value", panicMsg, "stack_trace", stackTrace)
}
