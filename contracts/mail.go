package contracts

import "context"

// MailMessage represents an email message.
type MailMessage struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	HtmlView    string
	TextView    string
	Attachments []string
}

// MailerContract defines the mail manager interface.
// Mirrors AdonisJS's @adonisjs/mail module.
type MailerContract interface {
	// Use returns a named mailer instance.
	Use(name string) MailerDriverContract

	// Send sends an email using the default mailer.
	Send(ctx context.Context, message MailMessage) error

	// SendLater queues an email to be sent later.
	SendLater(ctx context.Context, message MailMessage) error
}

// MailerDriverContract defines operations on a specific mail driver.
type MailerDriverContract interface {
	// Send sends the email.
	Send(ctx context.Context, message MailMessage) error
}
