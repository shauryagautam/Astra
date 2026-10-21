package database

import "fmt"

// ResolveDialect returns the appropriate dialect and driver name for a given connection string.
func ResolveDialect(driver string) (Dialect, string) {
	switch driver {
	case "neon":
		return NeonDialect{}, "pgx"
	case "postgres", "postgresql":
		return PostgresDialect{}, "pgx"
	case "mysql":
		return MySQLDialect{}, "mysql"
	case "sqlite", "sqlite3":
		return SQLiteDialect{}, "sqlite"
	default:
		return PostgresDialect{}, "pgx"
	}
}

// resolveDialect validates a driver for connection creation. ResolveDialect is
// retained for compatibility with callers that intentionally use the legacy
// postgres default.
func resolveDialect(driver string) (Dialect, string, error) {
	switch driver {
	case "", "postgres", "postgresql":
		return PostgresDialect{}, "pgx", nil
	case "neon":
		return NeonDialect{}, "pgx", nil
	case "mysql":
		return MySQLDialect{}, "mysql", nil
	case "sqlite", "sqlite3":
		return SQLiteDialect{}, "sqlite", nil
	default:
		return nil, "", fmt.Errorf("orm: unsupported database driver %q", driver)
	}
}
