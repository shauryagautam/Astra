package providers

import (
	"github.com/shauryagautam/Astra/pkg/engine"
)

type ValidateProvider struct {
	engine.BaseProvider
}

func (p *ValidateProvider) Name() string { return "validate" }

func (p *ValidateProvider) Register(app *engine.App) error {
	// Validation service should be wired via Wire.
	return nil
}

