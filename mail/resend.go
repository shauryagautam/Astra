package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/astraframework/astra/config"
)

// ResendMailer implements the Mailer interface using Resend.com.
type ResendMailer struct {
	config config.MailConfig
}

// NewResendMailer creates a new ResendMailer.
func NewResendMailer(cfg config.MailConfig) *ResendMailer {
	return &ResendMailer{config: cfg}
}

// Send sends an email via Resend HTTP API.
func (m *ResendMailer) Send(ctx context.Context, msg *Message) error {
	from := msg.From
	if from == "" {
		from = m.config.SMTPFrom
	}

	payload := map[string]any{
		"from":    from,
		"to":      msg.To,
		"subject": msg.Subject,
	}

	if msg.HTML != "" {
		payload["html"] = msg.HTML
	} else {
		payload["text"] = msg.Body
	}

	if len(msg.Attachments) > 0 {
		attachments := make([]map[string]any, 0, len(msg.Attachments))
		for _, a := range msg.Attachments {
			attachments = append(attachments, map[string]any{
				"filename": a.Name,
				"content":  a.Content, // Resend SDK/API usually handles []byte or requires base64
			})
		}
		payload["attachments"] = attachments
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+m.config.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("resend API returned status %d", res.StatusCode)
	}

	return nil
}
