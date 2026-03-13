package notification

import (
	"fmt"
	"log/slog"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/mail"
)

// Provider implements core.Provider for the notification package.
// It registers a *Notifier as "notifier" in the service container and
// automatically wires the mail channel if a Mailer is registered as "mailer".
//
// Example:
//
//	app.Use(notification.NewProvider())
//
// Access in handlers:
//
//	notifier := app.MustGet("notifier").(*notification.Notifier)
//	notifier.Send(ctx, &WelcomeNotification{User: user})
type Provider struct {
	core.BaseProvider
}

// NewProvider creates a notification Provider.
func NewProvider() *Provider { return &Provider{} }

// Register creates the Notifier and registers it.
func (p *Provider) Register(a *core.App) error {
	n := New()

	// Auto-wire mail channel if a Mailer is registered.
	if raw := a.Get("mailer"); raw != nil {
		if mailer, ok := raw.(mail.Mailer); ok {
			n.AddChannel(NewMailChannel(mailer))
			slog.Info("  notification: mail channel wired")
		}
	}

	a.Register("notifier", n)
	slog.Info("✓ Notifier registered")
	return nil
}

// ensure fmt is used.
var _ = fmt.Sprintf
