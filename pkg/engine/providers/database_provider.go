package providers

import (
	"github.com/shauryagautam/Astra/pkg/database"
	"context"
	"fmt"

	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/engine"
)

// DatabaseProvider implements engine.Provider for the Database service.
type DatabaseProvider struct {
	engine.BaseProvider
	db *database.DB
}

// ProvideDB is a static provider for the database.
func ProvideDB(env *config.Config) (*database.DB, error) {
	cfg := database.Config{
		Driver: env.String("DB_DRIVER", "postgres"),
		DSN:    env.String("DB_DSN", ""),
	}
	return database.Open(cfg)
}

// Register assembles the DB service into the app.
func (p *DatabaseProvider) Register(a *engine.App) error {
	dbService, err := ProvideDB(a.Env())
	if err != nil {
		return err
	}
	p.db = dbService

	a.RegisterHealthCheck("db", engine.HealthCheckFunc(func(ctx context.Context) error {
		if p.db == nil {
			return fmt.Errorf("db server: not initialized")
		}

		return p.db.Pool().PingContext(ctx)
	}))

	return nil
}


// Boot starts the database connection.
func (p *DatabaseProvider) Boot(a *engine.App) error {
	return nil
}

// Shutdown gracefully closes the database connection.
func (p *DatabaseProvider) Shutdown(ctx context.Context, a *engine.App) error {
	// Connection is closed in ORMProvider shutdown
	return nil
}
