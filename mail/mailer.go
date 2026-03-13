package mail

import "context"

// Attachment represents a file attached to an email.
type Attachment struct {
	Name    string
	Content []byte
	MIME    string
}

// Message represents an email message.
type Message struct {
	From        string
	To          []string
	Subject     string
	Body        string
	HTML        string
	Attachments []Attachment
}

// Mailer defines the interface for sending emails.
type Mailer interface {
	Send(ctx context.Context, msg *Message) error
}
