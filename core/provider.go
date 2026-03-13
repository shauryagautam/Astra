package core

import "context"

// Provider is the interface for Astra service providers.
type Provider interface {
	// Register services in the container.
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

func (p *BaseProvider) Register(a *App) error                      { return nil }
func (p *BaseProvider) Boot(a *App) error                          { return nil }
func (p *BaseProvider) Ready(a *App) error                         { return nil }
func (p *BaseProvider) Shutdown(ctx context.Context, a *App) error { return nil }
