package engine

import (
	"context"
	"log/slog"
	"strings"
)

// RedactingHandler wraps an existing slog.Handler and masks sensitive keys.
type RedactingHandler struct {
	slog.Handler
	SensitiveKeys []string
}

// NewRedactingHandler creates a new handler that redacts sensitive information.
func NewRedactingHandler(next slog.Handler, keys ...string) *RedactingHandler {
	if len(keys) == 0 {
		keys = []string{
			"password", "passcode", "token", "secret", "api_key",
			"authorization", "cookie", "set-cookie", "credit_card",
			"access_token", "refresh_token", "csrf",
		}
	}
	return &RedactingHandler{
		Handler:       next,
		SensitiveKeys: keys,
	}
}

// Handle redacts attributes before passing them to the next handler.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.Handler.Handle(ctx, r)
}

// Enabled implements slog.Handler.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithAttrs implements slog.Handler.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		if h.isSensitive(a.Key) {
			redacted[i] = slog.String(a.Key, "[REDACTED]")
		} else {
			redacted[i] = a
		}
	}
	return &RedactingHandler{
		Handler:       h.Handler.WithAttrs(redacted),
		SensitiveKeys: h.SensitiveKeys,
	}
}

// WithGroup implements slog.Handler.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		Handler:       h.Handler.WithGroup(name),
		SensitiveKeys: h.SensitiveKeys,
	}
}

func (h *RedactingHandler) isSensitive(key string) bool {
	key = strings.ToLower(key)
	for _, k := range h.SensitiveKeys {
		if key == k || strings.Contains(key, k) {
			return true
		}
	}
	return false
}

// ─── Context Helpers ──────────────────────────────────────────────────

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	TraceIDKey   contextKey = "trace_id"
)

// WithContext returns a logger that includes attributes from the context.
// It looks for request_id and trace_id.
func WithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if ctx == nil {
		return logger
	}

	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		logger = logger.With(slog.String("request_id", reqID))
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		logger = logger.With(slog.String("trace_id", traceID))
	}

	return logger
}

