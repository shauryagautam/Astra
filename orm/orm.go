package orm

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/astraframework/astra/orm/schema"
	"go.opentelemetry.io/otel/trace"
)

// DB wraps a database connection
type DB struct {
	conn    Connection
	dialect Dialect
	auditor Auditor
	pool    *sql.DB // Exposed for raw access and compatibility
}

func New(conn Connection, dialect Dialect) *DB {
	db := &DB{conn: conn, dialect: dialect}
	if sc, ok := conn.(*sqlConn); ok {
		db.pool = sc.db
	}
	return db
}

// Dialect returns the database dialect.
func (db *DB) Dialect() Dialect {
	return db.dialect
}

// Pool returns the underlying *sql.DB connection pool.
func (db *DB) Pool() *sql.DB {
	if db.pool != nil {
		return db.pool
	}
	// Fallback: try to unwrap from Connection if pool wasn't set
	type wrapper interface{ Unwrap() Connection }
	curr := db.conn
	for {
		if sc, ok := curr.(*sqlConn); ok {
			return sc.db
		}
		if w, ok := curr.(wrapper); ok {
			curr = w.Unwrap()
			continue
		}
		break
	}
	return nil
}

// Schema returns a schema builder
func (db *DB) Schema() *schema.Builder {
	return &schema.Builder{
		Dialect: db.dialect,
		Exec:    db,
	}
}

// Query is the public entry point for the ORM
func Query[T any](db *DB) *QueryBuilder[T] {
	return NewQueryBuilder[T](db)
}

// Open establishes a database connection and returns an ORM instance
func Open(cfg Config) (*DB, error) {
	var driverName string
	var dialect Dialect

	switch cfg.Driver {
	case "postgres", "postgresql":
		driverName = "pgx"
		dialect = PostgresDialect{}
	case "mysql":
		driverName = "mysql"
		dialect = MySQLDialect{}
	case "sqlite", "sqlite3":
		driverName = "sqlite"
		dialect = SQLiteDialect{}
	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}

	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, err
	}

	if cfg.MaxOpen > 0 {
		db.SetMaxOpenConns(cfg.MaxOpen)
	}
	if cfg.MaxIdle > 0 {
		db.SetMaxIdleConns(cfg.MaxIdle)
	}
	if cfg.Lifetime > 0 {
		db.SetConnMaxLifetime(cfg.Lifetime)
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	var conn Connection = &sqlConn{db}
	if cfg.Tracer != nil {
		conn = &tracingConn{inner: conn, tracer: cfg.Tracer}
	}
	if cfg.LogQueries {
		conn = &loggingConn{inner: conn}
	}

	return &DB{
		conn:    conn,
		dialect: dialect,
		auditor: cfg.Auditor,
		pool:    db,
	}, nil
}

// Close closes the underlying database pool.
func (db *DB) Close() error {
	if db.pool != nil {
		return db.pool.Close()
	}
	return nil
}

// Begin starts a new database transaction.
func (db *DB) Begin(ctx context.Context) (Transaction, error) {
	return db.conn.Begin(ctx)
}

// WithTx returns a new DB that runs queries inside the provided transaction.
// Use with a deferred rollback pattern to ensure cleanup:
//
//	tx, err := db.Begin(ctx)
//	if err != nil { ... }
//	defer tx.Rollback()
//	txDB := db.WithTx(tx)
//	// ... use txDB ...
//	tx.Commit()
func (db *DB) WithTx(tx Transaction) *DB {
	return &DB{
		conn:    tx,
		dialect: db.dialect,
		auditor: db.auditor,
		pool:    db.pool,
	}
}

// DropAllTables drops all tables in the current database.
// It handles foreign key constraints across different dialects.
func (db *DB) DropAllTables(ctx context.Context) error {
	var query string
	switch db.dialect.Name() {
	case "postgres":
		query = "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public'"
	case "mysql":
		query = "SHOW TABLES"
	case "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	default:
		return fmt.Errorf("orm: DropAllTables not supported for driver %s", db.dialect.Name())
	}

	rows, err := db.conn.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err == nil {
			tables = append(tables, table)
		}
	}

	// Disable foreign key checks if possible
	if db.dialect.Name() == "mysql" {
		if _, err := db.conn.Exec(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
			return err
		}
		defer func() { _, _ = db.conn.Exec(ctx, "SET FOREIGN_KEY_CHECKS = 1") }()
	} else if db.dialect.Name() == "sqlite" {
		if _, err := db.conn.Exec(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return err
		}
		defer func() { _, _ = db.conn.Exec(ctx, "PRAGMA foreign_keys = ON") }()
	}

	for _, table := range tables {
		var sqlStr string
		if db.dialect.Name() == "postgres" {
			sqlStr = fmt.Sprintf("DROP TABLE %s CASCADE", db.dialect.QuoteIdentifier(table))
		} else {
			sqlStr = fmt.Sprintf("DROP TABLE %s", db.dialect.QuoteIdentifier(table))
		}
		if _, err := db.conn.Exec(ctx, sqlStr); err != nil {
			return fmt.Errorf("orm: failed to drop table %s: %w", table, err)
		}
	}
	return nil
}

func (l *loggingConn) Begin(ctx context.Context) (Transaction, error) { return l.inner.Begin(ctx) }
func (l *loggingConn) Close() error                                   { return l.inner.Close() }

type loggingConn struct{ inner Connection }

func (l *loggingConn) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	slog.DebugContext(ctx, "orm.exec", "sql", sqlStr, "args", args)
	return l.inner.Exec(ctx, sqlStr, args...)
}
func (l *loggingConn) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	slog.DebugContext(ctx, "orm.query", "sql", sqlStr, "args", args)
	return l.inner.Query(ctx, sqlStr, args...)
}
func (l *loggingConn) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	slog.DebugContext(ctx, "orm.queryrow", "sql", sqlStr, "args", args)
	return l.inner.QueryRow(ctx, sqlStr, args...)
}

// Raw Query support
func (db *DB) Raw(sqlStr string, args ...any) *RawQuery {
	return &RawQuery{db: db, sql: sqlStr, args: args}
}

func (db *DB) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	return db.conn.Exec(ctx, sqlStr, args...)
}

type RawQuery struct {
	db   *DB
	sql  string
	args []any
}

// Scan scans raw SQL rows into dest.
// dest must be a *[]T where T is a struct, or a *T for a single row.
func (r *RawQuery) Scan(dest any, ctx ...context.Context) error {
	c := context.Background()
	if len(ctx) > 0 {
		c = ctx[0]
	}
	rows, err := r.db.conn.Query(c, r.sql, r.args...)
	if err != nil {
		return fmt.Errorf("orm: raw query: %w", err)
	}
	return scanInto(rows, dest)
}

// ScanSlice scans raw SQL rows into a *[]T.
func (r *RawQuery) ScanSlice(dest any, ctx ...context.Context) error {
	return r.Scan(dest, ctx...)
}

// ScanOne scans the first row of a raw SQL result into a *T.
func (r *RawQuery) ScanOne(dest any, ctx ...context.Context) error {
	c := context.Background()
	if len(ctx) > 0 {
		c = ctx[0]
	}
	rows, err := r.db.conn.Query(c, r.sql, r.args...)
	if err != nil {
		return fmt.Errorf("orm: raw query: %w", err)
	}
	return scanOneInto(rows, dest)
}

func (r *RawQuery) Rows(ctx ...context.Context) (Rows, error) {
	c := context.Background()
	if len(ctx) > 0 {
		c = ctx[0]
	}
	return r.db.conn.Query(c, r.sql, r.args...)
}

// sqlConn wraps sql.DB to satisfy Connection interface
type sqlConn struct {
	db *sql.DB
}

func (c *sqlConn) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	return c.db.ExecContext(ctx, sqlStr, args...)
}

func (c *sqlConn) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	rows, err := c.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows}, nil
}

func (c *sqlConn) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	return &sqlRow{c.db.QueryRowContext(ctx, sqlStr, args...)}
}

func (c *sqlConn) Begin(ctx context.Context) (Transaction, error) {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx}, nil
}

func (c *sqlConn) Close() error {
	return c.db.Close()
}

type sqlRows struct {
	*sql.Rows
}

func (r *sqlRows) Scan(dest ...any) error {
	return r.Rows.Scan(dest...)
}

func (r *sqlRows) Columns() ([]string, error) {
	return r.Rows.Columns()
}

type sqlRow struct {
	*sql.Row
}

func (r *sqlRow) Scan(dest ...any) error {
	return r.Row.Scan(dest...)
}

type sqlTx struct {
	*sql.Tx
}

func (t *sqlTx) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, sqlStr, args...)
}

func (t *sqlTx) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	rows, err := t.Tx.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows}, nil
}

func (t *sqlTx) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	return &sqlRow{t.Tx.QueryRowContext(ctx, sqlStr, args...)}
}

func (t *sqlTx) Begin(ctx context.Context) (Transaction, error) {
	return nil, fmt.Errorf("nested transactions not supported yet")
}

func (t *sqlTx) Commit() error {
	return t.Tx.Commit()
}

func (t *sqlTx) Rollback() error {
	return t.Tx.Rollback()
}

func (t *sqlTx) Close() error {
	return nil
}

// Config holds the database connection configuration.
type Config struct {
	Driver             string
	DSN                string
	MaxOpen            int
	MaxIdle            int
	Lifetime           time.Duration
	SlowQueryThreshold time.Duration
	Auditor            Auditor
	// Optional OpenTelemetry tracer. When set, all queries emit spans.
	Tracer trace.Tracer
	// LogQueries enables query logging to slog (development only).
	LogQueries bool
}

type Connection interface {
	Exec(ctx context.Context, sql string, args ...any) (sql.Result, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	Begin(ctx context.Context) (Transaction, error)
	Close() error
}

type Transaction interface {
	Connection
	Commit() error
	Rollback() error
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
	Columns() ([]string, error)
}

type Row interface {
	Scan(dest ...any) error
}

// Auditor defines the interface for custom audit loggers.
type Auditor interface {
	Audit(ctx context.Context, entry AuditEntry) error
}
