package runtime

import (
	"log/slog"
	"os"

	"github.com/google/wire"
	"github.com/shauryagautam/Astra/pkg/database"
	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
)

// ComponentSet is the Wire set for core application dependencies.
var ComponentSet = wire.NewSet(
	// Core Dependencies
	ProvideEnv,
	ProvideAstraConfig,
	ProvideLogger,

	// Application lifecycle manager.
	engine.New,

	// Authentication (OAuth2)
	ProvideOAuth2Manager,
)

// ProvideEnv loads the base environment configuration.
func ProvideEnv() (*config.Config, error) {
	return config.Load(".env")
}

// ProvideAstraConfig provides the typed application configuration.
func ProvideAstraConfig(env *config.Config) *config.AstraConfig {
	return config.LoadFromEnv(env)
}

// ProvideLogger provides the default application logger.
func ProvideLogger(cfg *config.AstraConfig) *slog.Logger {
	level := slog.LevelInfo
	if cfg.App.Debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// DataLayerSet provides database constructors.
var DataLayerSet = wire.NewSet(
	database.Open,
)
