package i18n

import (
	"context"
	"path/filepath"

	"github.com/astraframework/astra/core"
)

// Provider implements core.Provider for the I18n service.
type Provider struct {
	core.BaseProvider
	engine *Engine
}

// NewProvider creates a new I18n provider.
func NewProvider(fallback string) *Provider {
	return &Provider{
		engine: NewEngine(fallback),
	}
}

// Register assembles the i18n service into the container.
func (p *Provider) Register(a *core.App) error {
	a.Register("i18n", p.engine)
	return nil
}

// Boot loads translations from the resources/lang directory.
func (p *Provider) Boot(a *core.App) error {
	langDir := filepath.Join("resources", "lang")
	// Only load if the directory exists
	return p.engine.Load(langDir)
}

// Shutdown is a no-op for I18n.
func (p *Provider) Shutdown(ctx context.Context, a *core.App) error {
	return nil
}
