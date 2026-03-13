package database

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/events"
	"github.com/astraframework/astra/orm"
)

// DatabaseProvider implements core.Provider for the Database service.
type DatabaseProvider struct {
	core.BaseProvider
}

// Register assembles the DB service into the container.
func (p *DatabaseProvider) Register(a *core.App) error {
	emitter, _ := a.Get("events").(*events.Emitter)

	cfg := orm.Config{
		Driver: a.Env.String("DB_DRIVER", "postgres"),
		DSN:    a.Env.String("DB_DSN", ""),
	}

	dbService, err := orm.Open(cfg)
	if err != nil {
		return err
	}
	a.Register("db", dbService)

	a.RegisterHealthCheck("db", func(ctx context.Context) (error, map[string]any) {
		d, ok := a.Get("db").(*orm.DB)
		if !ok {
			return fmt.Errorf("db: service not found or not *orm.DB"), nil
		}

		stats := d.Pool().Stats()
		details := map[string]any{
			"max_open_connections": stats.MaxOpenConnections,
			"open_connections":     stats.OpenConnections,
			"in_use":               stats.InUse,
			"idle":                 stats.Idle,
			"wait_count":           stats.WaitCount,
			"wait_duration":        stats.WaitDuration.String(),
		}

		err := d.Pool().PingContext(ctx)
		return err, details
	})

	_ = emitter // placeholder

	return nil
}

// Boot starts the database connection.
func (p *DatabaseProvider) Boot(a *core.App) error {
	return nil
}

// Shutdown gracefully closes the database connection.
func (p *DatabaseProvider) Shutdown(ctx context.Context, a *core.App) error {
	// Connection is closed in ORMProvider shutdown
	return nil
}
