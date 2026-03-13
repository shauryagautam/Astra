package audit

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// AuditEvent represents a structured security or administrative event.
type AuditEvent struct {
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	Timestamp    time.Time `json:"timestamp"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
}

// Name implements events.Event.
func (e AuditEvent) Name() string {
	return "audit.event"
}

// Data implements events.Event.
func (e AuditEvent) Data() any {
	return e
}

// Auditor handles structured audit logging to a dedicated stream.
type Auditor struct {
	logger *slog.Logger
	file   *os.File
}

// NewAuditor creates a new Auditor that writes to the specified path.
func NewAuditor(logPath string) (*Auditor, error) {
	logPath = filepath.Clean(logPath)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	// Create a JSON handler that writes to the audit file
	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler).With("stream", "audit")

	return &Auditor{
		logger: logger,
		file:   f,
	}, nil
}

// Log records an audit event.
func (a *Auditor) Log(ctx context.Context, event AuditEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	a.logger.InfoContext(ctx, event.Action,
		"actor_id", event.ActorID,
		"resource_type", event.ResourceType,
		"resource_id", event.ResourceID,
		"ip_address", event.IPAddress,
		"user_agent", event.UserAgent,
		"success", event.Success,
		"error", event.Error,
	)
}

// Close closes the audit log file.
func (a *Auditor) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}
