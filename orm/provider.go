package orm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/telemetry"
)

// ORMProvider implements core.Provider for the ORM service.
// Register this provider in your application to connect to the database
// and make the *orm.DB instance available as "db".
//
// Example:
//
//	app.Use(&orm.ORMProvider{})
//
// Then in controllers or services:
//
//	db := app.MustGet("db").(*orm.DB)
//	users, err := orm.Query[User](db).Where("active", "=", true).Get(ctx)
type ORMProvider struct {
	core.BaseProvider
	db *DB
}

// Register connects to the database and registers the *orm.DB instance.
func (p *ORMProvider) Register(a *core.App) error {
	driver := detectDriver(a.Env.String("DB_DSN", ""))
	explicitDriver := a.Env.String("DB_DRIVER", "")
	if explicitDriver != "" {
		driver = explicitDriver
	}

	cfg := Config{
		Driver:     driver,
		DSN:        a.Env.String("DB_DSN", ""),
		MaxOpen:    a.Env.Int("DB_MAX_OPEN", 25),
		MaxIdle:    a.Env.Int("DB_MAX_IDLE", 5),
		Lifetime:   a.Env.Duration("DB_LIFETIME", 0),
		LogQueries: a.Env.Bool("DB_LOG_QUERIES", false),
	}

	if cfg.DSN == "" {
		return fmt.Errorf("orm: DB_DSN is not configured")
	}

	db, err := Open(cfg)
	if err != nil {
		return fmt.Errorf("orm: failed to connect: %w", err)
	}
	p.db = db

	slog.Info("✓ ORM connected", "driver", driver)
	a.Register("db", db)
	return nil
}

// Boot initializes ORM metrics if telemetry is enabled.
func (p *ORMProvider) Boot(a *core.App) error {
	if a.Env.String("OTEL_EXPORTER_OTLP_ENDPOINT", "") != "" || a.Env.String("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "") != "" {
		meter := telemetry.GetMeter()
		if err := ObservePool(p.db.Pool(), meter, detectDriver(a.Env.String("DB_DSN", ""))); err != nil {
			slog.Warn("orm: failed to register pool metrics", "error", err)
		}
	}
	return nil
}

// Shutdown closes the database connection pool.
func (p *ORMProvider) Shutdown(ctx context.Context, _ *core.App) error {
	if p.db != nil {
		if err := p.db.conn.Close(); err != nil {
			slog.ErrorContext(ctx, "orm: shutdown error", "error", err)
			return err
		}
	}
	return nil
}

// detectDriver infers the SQL driver name from a DSN URL prefix.
func detectDriver(dsn string) string {
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
