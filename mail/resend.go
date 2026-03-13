package mail

import (
	"bytes"
	"context"
	"fmt"
	"github.com/astraframework/astra/json"
	nethttp "net/http"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/events"
	"github.com/astraframework/astra/resilience"
)

// ResendMailer implements the Mailer interface using Resend.com.
type ResendMailer struct {
	config config.MailConfig
	events *events.Emitter
	cb     *resilience.CircuitBreaker
}

// NewResendMailer creates a new ResendMailer.
func NewResendMailer(cfg config.MailConfig, emitter *events.Emitter) *ResendMailer {
	return &ResendMailer{
		config: cfg,
		events: emitter,
		cb:     resilience.NewCircuitBreaker("mail:resend"),
	}
}

// Send sends an email via Resend HTTP API.
func (m *ResendMailer) Send(ctx context.Context, msg *Message) error {
	return m.cb.Execute(ctx, func() error {
		if msg == nil {
			return fmt.Errorf("mail: message is nil")
		}
		if len(msg.To) == 0 {
			return fmt.Errorf("mail: no recipients specified")
		}

		from := msg.From
		if from == "" {
			from = m.config.SMTPFrom
		}
		if from == "" {
			return fmt.Errorf("mail: from address is required")
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

		req, err := nethttp.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewBuffer(data))
		if err != nil {
			return err
		}

		req.Header.Set("Authorization", "Bearer "+m.config.ResendAPIKey)
		req.Header.Set("Content-Type", "application/json")

		client := &nethttp.Client{
			Timeout: 30 * time.Second,
		}
		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("mail: failed to send request: %w", err)
		}
		defer func() {
			if err := res.Body.Close(); err != nil {
				// Log close error
			}
		}()

		if res.StatusCode >= 400 {
			return fmt.Errorf("resend API returned status %d", res.StatusCode)
		}

		if m.events != nil {
			m.events.EmitPayload(ctx, "mail.sent", map[string]any{
				"driver":  "resend",
				"to":      msg.To,
				"subject": msg.Subject,
				"from":    from,
			})
		}

		return nil
	})
}
