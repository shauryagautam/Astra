package runtime

import (
	"log/slog"
	"os"

	"github.com/google/wire"
	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/database"
)

// ProviderSet is a Wire provider set that includes all core framework services.
var ProviderSet = wire.NewSet(
	// Core Dependencies
	ProvideEnv,
	ProvideAstraConfig,
	ProvideLogger,

	// App Container (Lifecycle Manager)
	engine.New,

	// Authentication (OAuth2)
	ProvideOAuth2Manager,
)

// ProvideEnv loads the base environment configuration.
func ProvideEnv() (*config.Config, error) {
	return config.Load(".env")
}

// ProvideAstraConfig provides the typed framework configuration.
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

// DataLayerSet provides database dependencies.
var DataLayerSet = wire.NewSet(
	database.Open,
)
