package core

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// DashboardEntry represents a single item in the dev dashboard.
type DashboardEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`     // "event", "log", "http", "query", "job", "mail", "redis"
	Category  string    `json:"category"` // more specific e.g. "auth", "db", "cache"
	Level     string    `json:"level,omitempty"`
	Message   string    `json:"message"`
	Data      any       `json:"data,omitempty"`
	Duration  int64     `json:"duration_ms,omitempty"`
}

// Dashboard manages the in-memory observability data for developers.
type Dashboard struct {
	mu      sync.RWMutex
	entries []DashboardEntry
	max     int
	counter int64
}

// NewDashboard creates a new Dashboard service.
func NewDashboard(maxEntries int) *Dashboard {
	if maxEntries <= 0 {
		maxEntries = 100 // Default
	}
	return &Dashboard{
		entries: make([]DashboardEntry, 0, maxEntries),
		max:     maxEntries,
	}
}

// Track adds a new entry to the dashboard.
func (d *Dashboard) Track(typ, message string, data any) {
	d.addEntry(DashboardEntry{
		Type:    typ,
		Message: message,
		Data:    data,
	})
}

// TrackRequest adds an HTTP request entry to the dashboard.
func (d *Dashboard) TrackRequest(method, path string, status int, duration time.Duration) {
	d.addEntry(DashboardEntry{
		Type:    "http",
		Message: method + " " + path,
		Data: map[string]any{
			"method":   method,
			"path":     path,
			"status":   status,
			"duration": duration.String(),
			"ms":       duration.Milliseconds(),
		},
		Duration: duration.Milliseconds(),
	})
}

// TrackLog adds a log entry to the dashboard.
func (d *Dashboard) TrackLog(level, message string, data any) {
	d.addEntry(DashboardEntry{
		Type:    "log",
		Level:   level,
		Message: message,
		Data:    data,
	})
}

// TrackQuery adds a database query entry to the dashboard.
func (d *Dashboard) TrackQuery(query string, args any, duration time.Duration) {
	d.addEntry(DashboardEntry{
		Type:     "query",
		Message:  query,
		Data:     args,
		Duration: duration.Milliseconds(),
	})
}

// TrackJob adds a background job entry to the dashboard.
func (d *Dashboard) TrackJob(name, status string, data any, duration time.Duration) {
	d.addEntry(DashboardEntry{
		Type:     "job",
		Category: status, // "started", "completed", "failed"
		Message:  name,
		Data:     data,
		Duration: duration.Milliseconds(),
	})
}

// TrackMail adds a mail entry to the dashboard.
func (d *Dashboard) TrackMail(to, subject string, data any) {
	d.addEntry(DashboardEntry{
		Type:    "mail",
		Message: subject,
		Data:    map[string]any{"to": to, "data": data},
	})
}

// TrackRedis adds a Redis command entry to the dashboard.
func (d *Dashboard) TrackRedis(command string, args any, duration time.Duration) {
	d.addEntry(DashboardEntry{
		Type:     "redis",
		Message:  command,
		Data:     args,
		Duration: duration.Milliseconds(),
	})
}

// TrackEvent adds a framework event entry to the dashboard.
func (d *Dashboard) TrackEvent(name string, payload any) {
	d.addEntry(DashboardEntry{
		Type:    "event",
		Message: name,
		Data:    payload,
	})
}

func (d *Dashboard) addEntry(entry DashboardEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.counter++
	entry.ID = d.counter
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	if len(d.entries) >= d.max {
		d.entries = d.entries[1:]
	}
	d.entries = append(d.entries, entry)
}

// Entries returns a copy of the recent dashboard entries.
func (d *Dashboard) Entries() []DashboardEntry {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make([]DashboardEntry, len(d.entries))
	copy(out, d.entries)
	return out
}

// Clear removes all entries from the dashboard.
func (d *Dashboard) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = d.entries[:0]
}

// LogHandler is a slog.Handler that pushes records to the dashboard.
type LogHandler struct {
	slog.Handler
	dash *Dashboard
}

// NewLogHandler wraps an existing slog.Handler.
func NewLogHandler(h slog.Handler, dash *Dashboard) *LogHandler {
	return &LogHandler{Handler: h, dash: dash}
}

// Handle implements slog.Handler.
func (h *LogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.dash.TrackLog(r.Level.String(), r.Message, nil)
	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new LogHandler with the given attributes added.
func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogHandler{Handler: h.Handler.WithAttrs(attrs), dash: h.dash}
}

// WithGroup returns a new LogHandler with the given group added.
func (h *LogHandler) WithGroup(name string) slog.Handler {
	return &LogHandler{Handler: h.Handler.WithGroup(name), dash: h.dash}
}
