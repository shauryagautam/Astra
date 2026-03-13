package orm

import "fmt"

// Dialect provides database-specific SQL generation
type Dialect interface {
	Name() string
	Placeholder(n int) string           // $1 (postgres) vs ? (mysql/sqlite)
	QuoteIdentifier(name string) string // "name" vs `name`
	SupportsReturning() bool
	AutoIncrementDDL() string
	LimitOffsetSQL(limit, offset int) string
	UpsertSQL(table string, columns []string, conflict string) string
	AdvisoryLock(id int64) string
	AdvisoryUnlock(id int64) string
}

// PostgresDialect implementation for PostgreSQL
type PostgresDialect struct{}

func (d PostgresDialect) Name() string                       { return "postgres" }
func (d PostgresDialect) Placeholder(n int) string           { return fmt.Sprintf("$%d", n) }
func (d PostgresDialect) QuoteIdentifier(name string) string { return fmt.Sprintf("\"%s\"", name) }
func (d PostgresDialect) SupportsReturning() bool            { return true }
func (d PostgresDialect) AutoIncrementDDL() string           { return "SERIAL" }
func (d PostgresDialect) LimitOffsetSQL(limit, offset int) string {
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
}
func (d PostgresDialect) UpsertSQL(table string, columns []string, conflict string) string {
	// Generic implementation for now
	return ""
}
func (d PostgresDialect) AdvisoryLock(id int64) string {
	return fmt.Sprintf("SELECT pg_advisory_lock(%d)", id)
}
func (d PostgresDialect) AdvisoryUnlock(id int64) string {
	return fmt.Sprintf("SELECT pg_advisory_unlock(%d)", id)
}

// MySQLDialect implementation for MySQL
type MySQLDialect struct{}

func (d MySQLDialect) Name() string                       { return "mysql" }
func (d MySQLDialect) Placeholder(n int) string           { return "?" }
func (d MySQLDialect) QuoteIdentifier(name string) string { return fmt.Sprintf("`%s`", name) }
func (d MySQLDialect) SupportsReturning() bool            { return false }
func (d MySQLDialect) AutoIncrementDDL() string           { return "AUTO_INCREMENT" }
func (d MySQLDialect) LimitOffsetSQL(limit, offset int) string {
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
}
func (d MySQLDialect) UpsertSQL(table string, columns []string, conflict string) string {
	return ""
}
func (d MySQLDialect) AdvisoryLock(id int64) string {
	return fmt.Sprintf("SELECT GET_LOCK('astra_migration_%d', 10)", id)
}
func (d MySQLDialect) AdvisoryUnlock(id int64) string {
	return fmt.Sprintf("SELECT RELEASE_LOCK('astra_migration_%d')", id)
}

// SQLiteDialect implementation for SQLite
type SQLiteDialect struct{}

func (d SQLiteDialect) Name() string                       { return "sqlite" }
func (d SQLiteDialect) Placeholder(n int) string           { return "?" }
func (d SQLiteDialect) QuoteIdentifier(name string) string { return fmt.Sprintf("`%s`", name) }
func (d SQLiteDialect) SupportsReturning() bool            { return false }
func (d SQLiteDialect) AutoIncrementDDL() string           { return "INTEGER PRIMARY KEY AUTOINCREMENT" }
func (d SQLiteDialect) LimitOffsetSQL(limit, offset int) string {
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
}
func (d SQLiteDialect) UpsertSQL(table string, columns []string, conflict string) string {
	return ""
}
func (d SQLiteDialect) AdvisoryLock(id int64) string {
	return "" // SQLite doesn't need advisory locks for single-file access usually, or doesn't support them.
}
func (d SQLiteDialect) AdvisoryUnlock(id int64) string {
	return ""
}
