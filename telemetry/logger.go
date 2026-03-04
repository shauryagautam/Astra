package telemetry

import (
	"log/slog"
	"os"

	"github.com/astraframework/astra/config"
)

// NewLogger creates a new slog logger based on the configuration.
func NewLogger(cfg config.AppConfig) *slog.Logger {
	var handler slog.Handler
	if cfg.Environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}
	return slog.New(handler)
}
