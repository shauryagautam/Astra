package providers

import (
	"log/slog"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/identity/auth"
)

// AuthProvider implements engine.Provider for the OAuth2 service.
type AuthProvider struct {
	engine.BaseProvider
	manager *auth.OAuth2Manager
}

func NewAuthProvider(m *auth.OAuth2Manager) *AuthProvider {
	return &AuthProvider{manager: m}
}

func (p *AuthProvider) Name() string { return "auth" }

func (p *AuthProvider) Register(a *engine.App) error {
	if p.manager == nil {
		slog.Warn("auth: manager is nil, auth service will not be available")
		return nil
	}
	
	slog.Info("✓ Auth service registered (OAuth2)")
	return nil
}
