package providers

import (
	"log/slog"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/mail"
	"github.com/shauryagautam/Astra/pkg/notification"
)

type NotificationProvider struct {
	engine.BaseProvider
	mailer mail.Mailer
}

func NewNotificationProvider(m mail.Mailer) *NotificationProvider {
	return &NotificationProvider{mailer: m}
}

func (p *NotificationProvider) Name() string { return "notification" }

func (p *NotificationProvider) Register(a *engine.App) error {
	n := notification.New()
	
	// Wire Mail Channel
	if p.mailer != nil {
		n.AddChannel(notification.NewMailChannel(p.mailer))
		slog.Info("notification: mail channel registered")
	}

	slog.Info("✓ notification service registered")
	return nil
}

