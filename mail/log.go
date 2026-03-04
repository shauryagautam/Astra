package mail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LogMailer implements the Mailer interface by writing emails to a local file/log.
type LogMailer struct {
	logDir string
}

// NewLogMailer creates a new LogMailer.
func NewLogMailer(logDir string) *LogMailer {
	return &LogMailer{logDir: logDir}
}

// Send writes the email to a file in the log directory.
func (m *LogMailer) Send(ctx context.Context, msg *Message) error {
	if err := os.MkdirAll(m.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create mail log directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_150405_000")
	filename := filepath.Join(m.logDir, fmt.Sprintf("mail_%s.log", timestamp))

	content := fmt.Sprintf("Date: %s\nFrom: %s\nTo: %v\nSubject: %s\n\nBody:\n%s\n\nHTML:\n%s\n\nAttachments: %d",
		time.Now().Format(time.RFC3339),
		msg.From,
		msg.To,
		msg.Subject,
		msg.Body,
		msg.HTML,
		len(msg.Attachments),
	)

	return os.WriteFile(filename, []byte(content), 0644)
}
