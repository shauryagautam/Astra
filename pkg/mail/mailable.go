package mail

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"

	"github.com/shauryagautam/Astra/pkg/queue"
)

// Mailable is the interface for structured, layout-aware HTML emails.
// Implement this to get automatic layout wrapping, data interpolation,
// and seamless integration with the notification package.
//
// Example:
//
//	type WelcomeMail struct {
//	    User schema.User
//	}
//
//	func (m *WelcomeMail) Subject() string         { return "Welcome to Astra!" }
//	func (m *WelcomeMail) From() string            { return "noreply@example.com" }
//	func (m *WelcomeMail) To() []string            { return []string{m.User.Email} }
//	func (m *WelcomeMail) Template() string        { return "emails/welcome" }
//	func (m *WelcomeMail) Data() map[string]any    { return map[string]any{"User": m.User} }
type Mailable interface {
	// Subject returns the email subject line.
	Subject() string
	// From returns the sender address (overrides mailer default if non-empty).
	From() string
	// To returns the list of recipient addresses.
	To() []string
	// Template returns the template name (without extension) relative to the
	// email template directory (e.g. "emails/welcome").
	Template() string
	// Data returns the template data map.
	Data() map[string]any
}

// MailableLayout optionally implemented by a Mailable to override the layout.
// If not implemented, the mailer's default layout is used.
type MailableLayout interface {
	Mailable
	// Layout returns the layout template name relative to the email template
	// directory (e.g. "layouts/email"). Return "" to disable layout.
	Layout() string
}

// MailableSender can send Mailable instances.
// Combine with TemplateMailer to get HTML rendering + delivery.
type MailableSender interface {
	SendMailable(m Mailable) error
}

// ─── TemplateMailer ────────────────────────────────────────────────────────────

// TemplateMailer wraps a base Mailer and adds HTML template rendering for Mailable.
type TemplateMailer struct {
	mailer        Mailer
	fs            fs.FS
	extension     string
	defaultFrom   string
	defaultLayout string
}

// TemplateMailerOption configures a TemplateMailer.
type TemplateMailerOption func(*TemplateMailer)

// WithMailFS sets the filesystem for email templates.
func WithMailFS(filesystem fs.FS) TemplateMailerOption {
	return func(tm *TemplateMailer) { tm.fs = filesystem }
}

// WithDefaultFrom sets the default sender address.
func WithDefaultFrom(from string) TemplateMailerOption {
	return func(tm *TemplateMailer) { tm.defaultFrom = from }
}

// WithDefaultLayout sets the default layout template for all emails.
// Set to "" to disable layout wrapping by default.
func WithDefaultLayout(layout string) TemplateMailerOption {
	return func(tm *TemplateMailer) { tm.defaultLayout = layout }
}

// WithMailExtension sets the template file extension (default: ".html").
func WithMailExtension(ext string) TemplateMailerOption {
	return func(tm *TemplateMailer) { tm.extension = ext }
}

// NewTemplateMailer creates a TemplateMailer that renders Mailable into HTML
// before handing off to the underlying Mailer.
func NewTemplateMailer(base Mailer, opts ...TemplateMailerOption) *TemplateMailer {
	tm := &TemplateMailer{
		mailer:    base,
		extension: ".html",
	}
	for _, o := range opts {
		o(tm)
	}
	return tm
}

// Send implements mail.Mailer so TemplateMailer can be used anywhere Mailer is expected.
// It delegates raw (pre-built) messages directly to the underlying mailer.
func (tm *TemplateMailer) Send(ctx context.Context, msg *Message) error {
	return tm.mailer.Send(ctx, msg)
}

// SendMailable renders the Mailable's template (with optional layout wrapping)
// and then returns the resulting message.
func (tm *TemplateMailer) SendMailable(m Mailable) (*Message, error) {
	html, err := tm.render(m)
	if err != nil {
		return nil, (fmt.Errorf("mail: render mailable: %w", err))
	}

	from := m.From()
	if from == "" {
		from = tm.defaultFrom
	}

	return &Message{
		From:    from,
		To:      m.To(),
		Subject: m.Subject(),
		HTML:    html,
	}, nil
}

// QueueMailable renders the mailable and returns a background job to send it.
func (tm *TemplateMailer) QueueMailable(m Mailable) (*queue.GenericJob[QueuedMailPayload], error) {
	msg, err := tm.SendMailable(m)
	if err != nil {
		return nil, err
	}
	return NewQueuedMail(msg, "default"), nil
}

// render produces the final HTML string for a Mailable.
func (tm *TemplateMailer) render(m Mailable) (string, error) {
	data := m.Data()
	if data == nil {
		data = make(map[string]any)
	}

	// Render the content template.
	contentHTML, err := tm.renderFile(m.Template()+tm.extension, data)
	if err != nil {
		return "", fmt.Errorf("mail: render template %q: %w", m.Template(), err)
	}

	// Determine layout.
	layout := tm.defaultLayout
	if ml, ok := m.(MailableLayout); ok {
		layout = ml.Layout()
	}

	if layout == "" {
		return contentHTML, nil
	}

	// Inject the rendered content into the layout via {{.Content}}.
	data["Content"] = template.HTML(contentHTML) // #nosec G203
	layoutHTML, err := tm.renderFile(layout+tm.extension, data)
	if err != nil {
		return "", fmt.Errorf("mail: render layout %q: %w", layout, err)
	}
	return layoutHTML, nil
}

// renderFile parses and executes a single template file.
func (tm *TemplateMailer) renderFile(name string, data any) (string, error) {
	var tmpl *template.Template
	var err error

	if tm.fs != nil {
		tmpl, err = template.ParseFS(tm.fs, name)
	} else {
		tmpl, err = template.ParseFiles(name)
	}
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	// Use the base filename as the template entry point.
	entry := filepath.Base(name)
	if err := tmpl.ExecuteTemplate(&buf, entry, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
