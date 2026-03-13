package mail

import (
	"context"
	"fmt"
	"github.com/astraframework/astra/json"

	"github.com/astraframework/astra/queue"
)

// QueuedMail represents a background job for sending an email.
type QueuedMail struct {
	queue.BaseJob
	Message *Message
	Driver  string // smtp, resend, log
}

// NewQueuedMail creates a new QueuedMail job.
func NewQueuedMail(msg *Message, driver string) *QueuedMail {
	return &QueuedMail{
		Message: msg,
		Driver:  driver,
	}
}

// Handle executes the mail sending logic in the background.
// Note: This requires access to the configured Mailer based on the Driver.
// In a real Astra app, the worker would use the service container to get the mailer.
func (j *QueuedMail) Handle(ctx context.Context) error {
	// This is a placeholder for how the worker would handle it.
	// Actual implementation would resolve the mailer from the container.
	fmt.Printf("Processing background mail to %v via %s\n", j.Message.To, j.Driver)
	return nil
}

// MarshalJSON ensures the job can be serialized for the queue.
func (j *QueuedMail) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"message": j.Message,
		"driver":  j.Driver,
	})
}
