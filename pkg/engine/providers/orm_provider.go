package providers

import (
	"github.com/shauryagautam/Astra/pkg/database"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/observability/metrics"
)

// ORMProvider implements engine.Provider for the ORM service.
// Register this provider in your application to connect to the database.
//
// In an idiomatic Go setup, use google/wire to inject the *database.DB 
// directly into handlers or repositories rather than resolving it from a container.
type ORMProvider struct {
	engine.BaseProvider
	db *database.DB
}

// Register connects to the database and registers the *database.DB instance.
func (p *ORMProvider) Register(a *engine.App) error {
	driver := detectORMDriver(a.Env().String("DB_DSN", ""))
	explicitDriver := a.Env().String("DB_DRIVER", "")
	if explicitDriver != "" {
		driver = explicitDriver
	}

	cfg := database.Config{
		Driver:     driver,
		DSN:        a.Env().String("DB_DSN", ""),
		MaxOpen:    a.Env().Int("DB_MAX_OPEN", 25),
		MaxIdle:    a.Env().Int("DB_MAX_IDLE", 5),
		Lifetime:   a.Env().Duration("DB_LIFETIME", 0),
		LogQueries: a.Env().Bool("DB_LOG_QUERIES", false),
	}

	if cfg.DSN == "" {
		return fmt.Errorf("orm: DB_DSN is not configured")
	}

	db, err := database.Open(cfg)
	if err != nil {
		return fmt.Errorf("orm: failed to connect: %w", err)
	}
	p.db = db

	slog.Info("✓ ORM connected", "driver", driver)
	return nil
}


// Boot initializes ORM metrics if telemetry is enabled.
func (p *ORMProvider) Boot(a *engine.App) error {
	if a.Env().String("OTEL_EXPORTER_OTLP_ENDPOINT", "") != "" || a.Env().String("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "") != "" {
		meter := metrics.GetMeter()
		if err := database.ObservePool(p.db.Pool(), meter, detectORMDriver(a.Env().String("DB_DSN", ""))); err != nil {
			slog.Warn("orm: failed to register pool metrics", "error", err)
		}
	}
	return nil
}

// Shutdown closes the database connection pool.
func (p *ORMProvider) Shutdown(ctx context.Context, _ *engine.App) error {
	if p.db != nil {
		if err := p.db.Close(); err != nil {
			slog.ErrorContext(ctx, "orm: shutdown error", "error", err)
			return err
		}
	}
	return nil
}

// detectORMDriver infers the SQL driver name from a DSN URL prefix.
func detectORMDriver(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return "postgres"
	case strings.HasPrefix(dsn, "mysql://"):
		return "mysql"
	case strings.HasPrefix(dsn, "sqlite://"), strings.HasPrefix(dsn, "file:"), dsn == ":memory:":
		return "sqlite"
	default:
		return "postgres"
	}
}
