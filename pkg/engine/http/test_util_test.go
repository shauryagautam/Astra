package http

import (
	"log/slog"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
)

// NewTestApp creates a minimal engine.App for testing HTTP components.
func NewTestApp() *engine.App {
	cfg := &config.AstraConfig{}
	env := &config.Config{}
	logger := slog.Default()
	return engine.New(cfg, env, logger)
}
