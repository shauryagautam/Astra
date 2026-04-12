package engine

import "context"

// Provider is the interface for Astra service providers.
type Provider interface {
	// Name returns the provider name for logging and debugging.
	Name() string

	// Register services in the application container.
	// Providers should use engine.Instance(a, service) or a.Container() to register
	// their managed services instead of setting fields on the App struct.
	Register(a *App) error

	// Boot the provider.
	Boot(a *App) error

	// Ready is called after all providers are booted.
	Ready(a *App) error

	// Shutdown gracefully stops the provider.
	Shutdown(ctx context.Context, a *App) error
}

// BaseProvider provides a default implementation for Provider.
type BaseProvider struct{}

func (p *BaseProvider) Name() string                               { return "unnamed" }
func (p *BaseProvider) Register(a *App) error                      { return nil }
func (p *BaseProvider) Boot(a *App) error                          { return nil }
func (p *BaseProvider) Ready(a *App) error                         { return nil }
func (p *BaseProvider) Shutdown(ctx context.Context, a *App) error { return nil }

// StandaloneProvider is a marker interface for service packages that are
// designed to be used both as Astra providers (via app.Use) AND as standalone
// libraries in standard net/http projects without importing engine.App.
//
// A package satisfies the standalone contract when:
//   1. It exposes a NewXxx(cfg XxxConfig) constructor that does NOT accept *App.
//   2. Its Register() method simply calls its own constructor and calls
//      app.Register(name, service) — no deep App coupling.
//   3. It can be compiled with only its own dependencies (no circular core import).
//
// Current standalone packages:
//   - github.com/shauryagautam/Astra/pkg/database   → orm.NewStandalone(cfg)
//   - github.com/shauryagautam/Astra/pkg/cache → cache.NewRedisStandalone(addr, pass, db)
//   - github.com/shauryagautam/Astra/pkg/auth  → auth.NewJWTStandalone(secret, opts...)
//
// To mark your provider as standalone, embed StandaloneProvider in its doc comment
// or implement this interface (it has no methods — it is purely a documentation tag).
type StandaloneProvider interface {
	// isStandalone is unexported to prevent accidental implementation.
	// Its presence on this interface is intentional: satisfy it only via embedding.
	Provider
}

