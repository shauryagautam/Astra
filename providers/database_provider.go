package providers

import (
	"fmt"

	"github.com/shaurya/astra/contracts"
	"github.com/shaurya/astra/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DatabaseProvider registers the database connection, migration runner,
// and seeder runner into the container.
// Mirrors Astra's DatabaseProvider.
type DatabaseProvider struct {
	BaseProvider
}

// NewDatabaseProvider creates a new DatabaseProvider.
func NewDatabaseProvider(app contracts.ApplicationContract) *DatabaseProvider {
	return &DatabaseProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register creates the GORM DB connection and binds it.
func (p *DatabaseProvider) Register() error {
	// Register the Database connection as a singleton
	p.App.Singleton("Astra/Lucid/Database", func(c contracts.ContainerContract) (any, error) {
		// Get DSN from config or env
		env := c.Use("Env").(*EnvManager)
		dsn := env.Get("DATABASE_URL", "")
		if dsn == "" {
			host := env.Get("DB_HOST", "127.0.0.1")
			port := env.Get("DB_PORT", "5432")
			user := env.Get("DB_USER", "postgres")
			password := env.Get("DB_PASSWORD", "")
			dbname := env.Get("DB_DATABASE", "astra_dev")
			sslmode := env.Get("DB_SSLMODE", "disable")
			dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
				host, port, user, password, dbname, sslmode)
		}

		logLevel := logger.Silent
		if env.Get("DB_DEBUG", "false") == "true" {
			logLevel = logger.Info
		}

		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logLevel),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		return db, nil
	})
	p.App.Alias("Database", "Astra/Lucid/Database")
	p.App.Alias("DB", "Astra/Lucid/Database")

	// Register Migration Runner
	p.App.Singleton("Astra/Lucid/Migration", func(c contracts.ContainerContract) (any, error) {
		db := c.Use("Database").(*gorm.DB)
		return database.NewMigrationRunner(db), nil
	})
	p.App.Alias("Migration", "Astra/Lucid/Migration")

	// Register Seeder Runner
	p.App.Singleton("Astra/Lucid/Seeder", func(c contracts.ContainerContract) (any, error) {
		db := c.Use("Database").(*gorm.DB)
		return database.NewSeederRunner(db), nil
	})
	p.App.Alias("Seeder", "Astra/Lucid/Seeder")

	return nil
}

// Shutdown closes the database connection.
func (p *DatabaseProvider) Shutdown() error {
	if p.App.HasBinding("Database") {
		db, err := p.App.Make("Database")
		if err == nil {
			if gormDB, ok := db.(*gorm.DB); ok {
				sqlDB, err := gormDB.DB()
				if err == nil {
					return sqlDB.Close()
				}
			}
		}
	}
	return nil
}
