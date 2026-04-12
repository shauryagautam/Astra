package mail

import (
	"github.com/shauryagautam/Astra/pkg/queue"
)

// QueuedMailPayload holds the data for a background email task.
type QueuedMailPayload struct {
	Message *Message
	Driver  string
}

// NewQueuedMail creates a new background job for sending an email.
func NewQueuedMail(msg *Message, driver string) *queue.GenericJob[QueuedMailPayload] {
	return queue.NewJob("mail.send", QueuedMailPayload{
		Message: msg,
		Driver:  driver,
	})
}
