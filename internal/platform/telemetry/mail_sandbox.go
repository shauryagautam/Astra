package platformtelemetry

import (
	"github.com/shauryagautam/Astra/pkg/engine/telemetry"
	"sync"
	"time"
)

// SandboxMail holds a captured outbound email for inspection in the Astra Cockpit
// Mail Sandbox panel. When the MailSandbox is active, no real SMTP call is made.
type SandboxMail struct {
	ID        int64     `json:"id"`
	To        []string  `json:"to"`
	Subject   string    `json:"subject"`
	HTML      string    `json:"html,omitempty"`
	Text      string    `json:"text,omitempty"`
	FromAddr  string    `json:"from,omitempty"`
	SentAt    time.Time `json:"sent_at"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// MailSandbox is an in-process mail trap that captures sent emails without
// delivering them via SMTP. It is designed to be swapped in as the active
// mail driver when APP_ENV != "production".
//
// Usage in your app bootstrap:
//
//	if !app.Env.IsProd() {
//	    sandbox := framework.NewMailSandbox(app.telemetry.Dashboard, 200)
//	    app.Register("mailer", sandbox)
//	}
type MailSandbox struct {
	mu        sync.RWMutex
	mails     []SandboxMail
	max       int
	counter   int64
	dashboard *telemetry.Dashboard // optional — when set, mails are also tracked in the main dashboard
}

// NewMailSandbox creates a MailSandbox with a ring-buffer of maxEmails capacity.
// If dash is non-nil, every captured mail is also forwarded to the dashboard's
// TrackMail method for the unified entry stream.
func NewMailSandbox(dash *telemetry.Dashboard, maxEmails int) *MailSandbox {
	if maxEmails <= 0 {
		maxEmails = 200
	}
	return &MailSandbox{
		mails:     make([]SandboxMail, 0, maxEmails),
		max:       maxEmails,
		dashboard: dash,
	}
}

// Capture stores an outbound email in the sandbox ring-buffer.
// This method is the integration point for mail driver adapters.
func (s *MailSandbox) Capture(m SandboxMail) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	m.ID = s.counter
	if m.SentAt.IsZero() {
		m.SentAt = time.Now()
	}

	if len(s.mails) >= s.max {
		s.mails = s.mails[1:] // evict oldest
	}
	s.mails = append(s.mails, m)

	// Forward to the global dashboard entry stream (non-blocking).
	if s.dashboard != nil {
		to := ""
		if len(m.To) > 0 {
			to = m.To[0]
		}
		s.dashboard.TrackMail(to, m.Subject, map[string]any{
			"id":      m.ID,
			"to":      m.To,
			"html":    len(m.HTML) > 0,
			"text":    len(m.Text) > 0,
		})
	}
}

// Mails returns a snapshot of all captured emails (newest last).
func (s *MailSandbox) Mails() []SandboxMail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SandboxMail, len(s.mails))
	copy(out, s.mails)
	return out
}

// GetByID returns a single mail by ID or (zero, false) if not found.
func (s *MailSandbox) GetByID(id int64) (SandboxMail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mails {
		if m.ID == id {
			return m, true
		}
	}
	return SandboxMail{}, false
}

// Clear removes all captured emails.
func (s *MailSandbox) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mails = s.mails[:0]
}

// Count returns the number of emails currently held in the sandbox.
func (s *MailSandbox) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.mails)
}
