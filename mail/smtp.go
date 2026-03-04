package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/astraframework/astra/config"
)

// SMTPMailer implements the Mailer interface using SMTP.
type SMTPMailer struct {
	config config.MailConfig
}

// NewSMTPMailer creates a new SMTPMailer.
func NewSMTPMailer(cfg config.MailConfig) *SMTPMailer {
	return &SMTPMailer{config: cfg}
}

// Send sends an email using SMTP.
func (m *SMTPMailer) Send(ctx context.Context, msg *Message) error {
	auth := smtp.PlainAuth("", m.config.SMTPUser, m.config.SMTPPassword, m.config.SMTPHost)

	from := msg.From
	if from == "" {
		from = m.config.SMTPFrom
	}

	dest := strings.Join(msg.To, ",")

	var body bytes.Buffer
	body.WriteString(fmt.Sprintf("To: %s\r\n", dest))
	body.WriteString(fmt.Sprintf("From: %s\r\n", from))
	body.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))

	if len(msg.Attachments) == 0 {
		if msg.HTML != "" {
			body.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
			body.WriteString(msg.HTML)
		} else {
			body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
			body.WriteString(msg.Body)
		}
	} else {
		boundary := "astra_mail_boundary"
		body.WriteString("MIME-Version: 1.0\r\n")
		body.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary))

		// Body part
		body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		if msg.HTML != "" {
			body.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
			body.WriteString(msg.HTML)
		} else {
			body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
			body.WriteString(msg.Body)
		}
		body.WriteString("\r\n")

		// Attachments
		for _, a := range msg.Attachments {
			body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			mime := a.MIME
			if mime == "" {
				mime = "application/octet-stream"
			}
			body.WriteString(fmt.Sprintf("Content-Type: %s\r\n", mime))
			body.WriteString("Content-Transfer-Encoding: base64\r\n")
			body.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", a.Name))

			encoder := base64.NewEncoder(base64.StdEncoding, &body)
			encoder.Write(a.Content)
			encoder.Close()
			body.WriteString("\r\n")
		}
		body.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	}

	addr := fmt.Sprintf("%s:%d", m.config.SMTPHost, m.config.SMTPPort)
	err := smtp.SendMail(addr, auth, from, msg.To, body.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send smtp mail: %w", err)
	}

	return nil
}
