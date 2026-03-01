package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/shaurya/astra/contracts"
)

// SMTPConfig holds settings for an SMTP connection.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPDriver implements the MailerDriverContract for SMTP.
type SMTPDriver struct {
	config SMTPConfig
}

// NewSMTPDriver creates a new SMTPDriver.
func NewSMTPDriver(config SMTPConfig) *SMTPDriver {
	return &SMTPDriver{config: config}
}

func (s *SMTPDriver) Send(ctx context.Context, message contracts.MailMessage) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// Build the email message
	to := strings.Join(message.To, ", ")
	from := message.From
	if from == "" {
		from = s.config.From
	}

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", to, message.Subject, message.HtmlView))

	if message.TextView != "" && message.HtmlView == "" {
		msg = []byte(fmt.Sprintf("To: %s\r\n"+
			"Subject: %s\r\n"+
			"\r\n"+
			"%s\r\n", to, message.Subject, message.TextView))
	}

	return smtp.SendMail(addr, auth, from, message.To, msg)
}
