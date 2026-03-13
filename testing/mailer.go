package testing

import (
	"context"
	"testing"

	"github.com/astraframework/astra/mail"
	"github.com/stretchr/testify/assert"
)

// FakeMailer is a test mailer that collects sent messages in memory.
type FakeMailer struct {
	Messages []*mail.Message
}

// NewFakeMailer creates a new FakeMailer.
func NewFakeMailer() *FakeMailer {
	return &FakeMailer{
		Messages: make([]*mail.Message, 0),
	}
}

// Send implements the mail.Mailer interface.
func (m *FakeMailer) Send(ctx context.Context, msg *mail.Message) error {
	m.Messages = append(m.Messages, msg)
	return nil
}

// AssertSent asserts that an email was sent to the given address.
func (m *FakeMailer) AssertSent(t *testing.T, to string) {
	t.Helper()
	for _, msg := range m.Messages {
		for _, recipient := range msg.To {
			if recipient == to {
				return
			}
		}
	}
	assert.Fail(t, "Email was not sent to missing address", "Expected email sent to %s", to)
}

// AssertNotSent asserts that no emails were sent.
func (m *FakeMailer) AssertNotSent(t *testing.T) {
	t.Helper()
	assert.Empty(t, m.Messages, "Expected no emails to be sent, but %d were sent", len(m.Messages))
}
