package config

// DatabaseConfig holds PostgreSQL database configuration.
// Mirrors AdonisJS's config/database.ts.
type DatabaseConfig struct {
	// Connection specifies the default connection (e.g., "pg").
	Connection string

	// PostgreSQL connection details
	PG PostgresConfig
}

// PostgresConfig holds PostgreSQL-specific settings.
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string

	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // seconds

	// Debug enables SQL query logging.
	Debug bool
}

// DefaultDatabaseConfig returns sensible defaults.
func DefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Connection: "pg",
		PG: PostgresConfig{
			Host:            "127.0.0.1",
			Port:            5432,
			User:            "postgres",
			Password:        "",
			Database:        "adonis_dev",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 300,
			Debug:           false,
		},
	}
}
