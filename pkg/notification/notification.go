package notification

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/shauryagautam/Astra/pkg/mail"
)

// Notification is the interface every notification must implement.
// Via() returns the list of channels to send on (e.g. "mail", "sms", "slack").
type Notification interface {
	Via() []string
}

// MailableNotification is implemented by notifications that send an email.
type MailableNotification interface {
	Notification
	ToMail() *mail.Message
}

// Channel is a pluggable delivery backend.
type Channel interface {
	// Name returns the channel identifier (e.g. "mail", "sms").
	Name() string
	// Send delivers the notification.
	Send(ctx context.Context, n Notification) error
}

// Notifier is the central notification dispatcher.
// Register channels via AddChannel, then call Send.
//
// Example:
//
//	n := notification.New()
//	n.AddChannel(notification.NewMailChannel(mailer))
//	n.Send(ctx, &WelcomeNotification{User: user})
type Notifier struct {
	mu       sync.RWMutex
	channels map[string]Channel
}

// New creates a new Notifier with no channels.
func New() *Notifier {
	return &Notifier{
		channels: make(map[string]Channel),
	}
}

// AddChannel registers a delivery channel.
func (n *Notifier) AddChannel(ch Channel) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.channels[ch.Name()] = ch
}

// Send dispatches a notification on all channels returned by n.Via().
// Errors are logged but do not cause Send to fail fast — all channels are tried.
func (n *Notifier) Send(ctx context.Context, notif Notification) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var errs []error
	for _, via := range notif.Via() {
		ch, ok := n.channels[via]
		if !ok {
			slog.WarnContext(ctx, "notification: unknown channel", "channel", via)
			continue
		}
		if err := ch.Send(ctx, notif); err != nil {
			slog.ErrorContext(ctx, "notification: send failed", "channel", via, "error", err)
			errs = append(errs, fmt.Errorf("channel %q: %w", via, err))
		}
	}

	if len(errs) > 0 {
		return errs[0] // return first error (caller can inspect logs for all)
	}
	return nil
}

// SendAll dispatches notifications to a list of recipients.
// Each recipient is passed to recipientFn to produce the Notification.
func (n *Notifier) SendAll(ctx context.Context, notif Notification) error {
	return n.Send(ctx, notif)
}

// Notify implements engine.Notifier.
func (n *Notifier) Notify(ctx context.Context, notif any) error {
	v, ok := notif.(Notification)
	if !ok {
		return fmt.Errorf("notification: object does not implement Notification interface")
	}
	return n.Send(ctx, v)
}

// ─── Mail Channel ──────────────────────────────────────────────────────────────

// MailChannel delivers notifications over email.
type MailChannel struct {
	mailer mail.Mailer
}

// NewMailChannel creates a MailChannel backed by the given Mailer.
func NewMailChannel(mailer mail.Mailer) *MailChannel {
	return &MailChannel{mailer: mailer}
}

func (c *MailChannel) Name() string { return "mail" }

func (c *MailChannel) Send(ctx context.Context, n Notification) error {
	mn, ok := n.(MailableNotification)
	if !ok {
		return fmt.Errorf("notification: not a MailableNotification")
	}
	msg := mn.ToMail()
	if msg == nil {
		return fmt.Errorf("notification: ToMail() returned nil")
	}
	return c.mailer.Send(ctx, msg)
}

// ─── Database Channel ─────────────────────────────────────────────────────────

// DatabaseNotification is implemented by notifications that persist to a DB.
type DatabaseNotification interface {
	Notification
	// ToDatabase returns the data to store in the notifications table.
	ToDatabase() map[string]any
}

// DatabaseChannel persists notifications to a database via a user-supplied writer.
type DatabaseChannel struct {
	writer DatabaseWriter
}

// DatabaseWriter is implemented by the application's notification repository.
type DatabaseWriter interface {
	Write(ctx context.Context, data map[string]any) error
}

// NewDatabaseChannel creates a DatabaseChannel.
func NewDatabaseChannel(writer DatabaseWriter) *DatabaseChannel {
	return &DatabaseChannel{writer: writer}
}

func (c *DatabaseChannel) Name() string { return "database" }

func (c *DatabaseChannel) Send(ctx context.Context, n Notification) error {
	dn, ok := n.(DatabaseNotification)
	if !ok {
		return fmt.Errorf("notification: not a DatabaseNotification")
	}
	return c.writer.Write(ctx, dn.ToDatabase())
}
