package providers

import (
	"context"
	"path/filepath"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/i18n"
)

type I18nProvider struct {
	engine.BaseProvider
	i18n *i18n.Engine
}

func NewI18nProvider(fallback string) *I18nProvider {
	return &I18nProvider{
		i18n: i18n.NewEngine(fallback),
	}
}

func (p *I18nProvider) Name() string { return "i18n" }

func (p *I18nProvider) Register(a *engine.App) error {
	// I18n translator should be wired via Wire.
	return nil
}


func (p *I18nProvider) Boot(a *engine.App) error {
	langDir := filepath.Join("resources", "lang")
	return p.i18n.Load(langDir)
}

func (p *I18nProvider) Shutdown(ctx context.Context, a *engine.App) error {
	return nil
}
