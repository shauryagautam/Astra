package database

// AttachmentResolver allows the ORM to resolve attachment URLs
// without depending directly on the storage package.
var AttachmentResolver func(disk, path string) (string, error)

// SetAttachmentResolver sets the global attachment resolver.
func SetAttachmentResolver(fn func(disk, path string) (string, error)) {
	AttachmentResolver = fn
}

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
