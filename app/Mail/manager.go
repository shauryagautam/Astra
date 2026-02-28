package mail

import (
	"context"
	"fmt"
	"sync"

	"github.com/shaurya/adonis/contracts"
)

// MailManager manages multiple mailers and provides a default mailer.
type MailManager struct {
	mu            sync.RWMutex
	mailers       map[string]contracts.MailerDriverContract
	defaultMailer string
	queue         contracts.QueueContract
}

// NewMailManager creates a new MailManager.
func NewMailManager(defaultMailer string, queue contracts.QueueContract) *MailManager {
	return &MailManager{
		mailers:       make(map[string]contracts.MailerDriverContract),
		defaultMailer: defaultMailer,
		queue:         queue,
	}
}

// Register registers a named mailer driver.
func (m *MailManager) Register(name string, driver contracts.MailerDriverContract) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mailers[name] = driver
}

// Use returns a named mailer instance.
func (m *MailManager) Use(name string) contracts.MailerDriverContract {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mailer, ok := m.mailers[name]
	if !ok {
		panic(fmt.Sprintf("Mailer '%s' not registered", name))
	}
	return mailer
}

// Default returns the default mailer.
func (m *MailManager) Default() contracts.MailerDriverContract {
	return m.Use(m.defaultMailer)
}

// Send sends an email using the default mailer.
func (m *MailManager) Send(ctx context.Context, message contracts.MailMessage) error {
	return m.Default().Send(ctx, message)
}

// SendLater queues an email to be sent later using the Queue system.
func (m *MailManager) SendLater(ctx context.Context, message contracts.MailMessage) error {
	if m.queue == nil {
		return fmt.Errorf("queue system not available for SendLater")
	}

	job := &MailJob{Message: message, Mailer: m.defaultMailer}
	return m.queue.Push(job)
}

// Ensure MailManager implements MailerContract.
var _ contracts.MailerContract = (*MailManager)(nil)
