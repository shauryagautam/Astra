package providers

import (
	"log/slog"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/identity/rbac"
)

type RBACProvider struct {
	engine.BaseProvider
	rbac *rbac.RBAC
}

func NewRBACProvider(r *rbac.RBAC) *RBACProvider {
	return &RBACProvider{rbac: r}
}

func (p *RBACProvider) Name() string { return "rbac" }

func (p *RBACProvider) Register(a *engine.App) error {
	slog.Info("✓ RBAC service registered")
	return nil
}

