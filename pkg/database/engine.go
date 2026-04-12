package database

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/shauryagautam/Astra/pkg/database/schema"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	tenantIDKey contextKey = "astra_tenant_id"
)

// WithTenantID returns a new context with the tenant ID attached.
func WithTenantID(ctx context.Context, tenantID any) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantIDFromContext retrieves the tenant ID from the context.
func TenantIDFromContext(ctx context.Context) (any, bool) {
	val := ctx.Value(tenantIDKey)
	return val, val != nil
}

// QueryHook is called after every SQL statement with the query, bound arguments,
// and how long the statement took to execute. It is safe to call from multiple
// goroutines concurrently.
//
// Use this hook to feed the Astra Cockpit SQL Timeline panel without importing
// the core package from within the ORM:
//
//	cfg.QueryHook = func(sql string, args []any, d time.Duration) {
//	    dashboard.TrackQuery(sql, args, d)
//	}
type QueryHook func(sql string, args []any, duration time.Duration)


// DB wraps a database connection
type DB struct {
	conn    Connection
	dialect Dialect
	auditor Auditor
	pool    *sql.DB // Exposed for raw access and compatibility
	inTx    bool
}

func New(conn Connection, dialect Dialect) *DB {
	db := &DB{conn: conn, dialect: dialect}
	if sc, ok := conn.(*sqlConn); ok {
		db.pool = sc.db
	}
	return db
}

// SetQueryHook sets a hook that is called after every query.
func (db *DB) SetQueryHook(hook QueryHook) {
	db.conn = &dashboardConn{inner: db.conn, hook: hook}
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

// Query is the public entry point for the ORM.
// It automatically detects if a transaction is present in the context and uses it.
func Query[T any](db *DB, ctx ...context.Context) *QueryBuilder[T] {
	activeDB := db
	var activeCtx context.Context
	if len(ctx) > 0 && ctx[0] != nil {
		activeCtx = ctx[0]
		if txDB, ok := FromContext(activeCtx); ok {
			activeDB = txDB
		}
	}
	qb := NewQueryBuilder[T](activeDB)
	if activeCtx != nil {
		qb.ctx = activeCtx
	}
	return qb
}

// Open establishes a database connection and returns an ORM instance
func Open(cfg Config) (*DB, error) {
	dialect, driverName := ResolveDialect(cfg.Driver)

	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, err
	}

	// Apply dialect-specific pool configuration (e.g., Neon serverless limits)
	dialect.ConfigurePool(db)

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
	if dialect.Name() == "neon" {
		conn = &neonConn{inner: conn}
	}
	if cfg.Tracer != nil {
		conn = &tracingConn{inner: conn, tracer: cfg.Tracer}
	}
	if cfg.LogQueries {
		conn = &loggingConn{inner: conn}
	}
	if cfg.QueryHook != nil {
		conn = &dashboardConn{inner: conn, hook: cfg.QueryHook}
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
		inTx:    true,
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

	if len(tables) == 0 {
		return nil
	}

	if db.dialect.Name() == "postgres" {
		var quoted []string
		for _, t := range tables {
			quoted = append(quoted, db.dialect.QuoteIdentifier(t))
		}
		// Batched drop for postgres
		sqlStr := fmt.Sprintf("DROP TABLE %s CASCADE", strings.Join(quoted, ", "))
		if _, err := db.conn.Exec(ctx, sqlStr); err != nil {
			return fmt.Errorf("orm: failed to drop tables: %w", err)
		}
	} else {
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

// Raw Query support is now provided via the generic Raw[T] function.
// func (db *DB) Raw(sqlStr string, args ...any) *RawQuery[any] { ... } is temporarily removed
// to encourage migration to Raw[T].

// Exec executes a query without returning any rows.
// It automatically detects if a transaction is present in the context and uses it.
func (db *DB) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	activeDB := db
	if txDB, ok := FromContext(ctx); ok {
		activeDB = txDB
	}
	return activeDB.conn.Exec(ctx, sqlStr, args...)
}

// Query executes a query that returns rows.
// It automatically detects if a transaction is present in the context and uses it.
func (db *DB) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	activeDB := db
	if txDB, ok := FromContext(ctx); ok {
		activeDB = txDB
	}
	return activeDB.conn.Query(ctx, sqlStr, args...)
}

// QueryRow executes a query that is expected to return at most one row.
// It automatically detects if a transaction is present in the context and uses it.
func (db *DB) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	activeDB := db
	if txDB, ok := FromContext(ctx); ok {
		activeDB = txDB
	}
	return activeDB.conn.QueryRow(ctx, sqlStr, args...)
}

type RawQuery[T any] struct {
	db   *DB
	sql  string
	args []any
}

// Raw creates a raw SQL query that can be scanned into T.
func Raw[T any](db *DB, sqlStr string, args ...any) *RawQuery[T] {
	return &RawQuery[T]{db: db, sql: sqlStr, args: args}
}

// Scan scans raw SQL rows into dest.
// dest must be a *[]T where T is a struct, or a *T for a single row.
func (r *RawQuery[T]) Scan(dest any, ctx ...context.Context) error {
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
func (r *RawQuery[T]) ScanSlice(dest any, ctx ...context.Context) error {
	return r.Scan(dest, ctx...)
}

// ScanOne scans the first row of a raw SQL result into a *T.
func (r *RawQuery[T]) ScanOne(dest any, ctx ...context.Context) error {
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

func (r *RawQuery[T]) Rows(ctx ...context.Context) (Rows, error) {
	c := context.Background()
	if len(ctx) > 0 {
		c = ctx[0]
	}
	return r.db.conn.Query(c, r.sql, r.args...)
}

// All returns an iterator over the raw query results, scanning into T.
func (r *RawQuery[T]) All(ctx ...context.Context) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		c := context.Background()
		if len(ctx) > 0 {
			c = ctx[0]
		}
		rows, err := r.db.conn.Query(c, r.sql, r.args...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()

		var zero T
		t := reflect.TypeOf(zero)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		
		// If T is a struct, use model meta for mapping columns
		if t.Kind() == reflect.Struct {
			meta := GetMeta(t)
			columns, err := rows.Columns()
			if err != nil {
				yield(nil, err)
				return
			}

			colMetas := make([]ColumnMeta, len(columns))
			colValid := make([]bool, len(columns))
			for i, name := range columns {
				if cm, ok := meta.ColumnByCol[name]; ok {
					colMetas[i] = cm
					colValid[i] = true
				}
			}

			for rows.Next() {
				var item T
				v := reflect.ValueOf(&item)
				if v.Kind() == reflect.Ptr {
					v = v.Elem()
				}
				if err := r.db.scanRow(rows, columns, colMetas, colValid, v); err != nil {
					if !yield(nil, err) {
						return
					}
					continue
				}
				if !yield(&item, nil) {
					return
				}
			}
		} else {
			// Primitive types (single column)
			for rows.Next() {
				var item T
				if err := rows.Scan(&item); err != nil {
					if !yield(nil, err) {
						return
					}
					continue
				}
				if !yield(&item, nil) {
					return
				}
			}
		}

		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
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
	spID := uuid.NewString()
	spName := "astra_sp_" + strings.ReplaceAll(spID, "-", "_")
	if _, err := t.Tx.ExecContext(ctx, "SAVEPOINT "+spName); err != nil {
		return nil, fmt.Errorf("nested transactions failed to create savepoint: %w", err)
	}
	return &savepointTx{tx: t.Tx, savepoint: spName}, nil
}

type savepointTx struct {
	tx        *sql.Tx
	savepoint string
}

func (s *savepointTx) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	return s.tx.ExecContext(ctx, sqlStr, args...)
}

func (s *savepointTx) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	rows, err := s.tx.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRows{rows}, nil
}

func (s *savepointTx) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	return &sqlRow{s.tx.QueryRowContext(ctx, sqlStr, args...)}
}

func (s *savepointTx) Begin(ctx context.Context) (Transaction, error) {
	spID := uuid.NewString()
	spName := "astra_sp_" + strings.ReplaceAll(spID, "-", "_")
	if _, err := s.tx.ExecContext(ctx, "SAVEPOINT "+spName); err != nil {
		return nil, err
	}
	return &savepointTx{tx: s.tx, savepoint: spName}, nil
}

func (s *savepointTx) Commit() error {
	_, err := s.tx.ExecContext(context.Background(), "RELEASE SAVEPOINT "+s.savepoint)
	return err
}

func (s *savepointTx) Rollback() error {
	_, err := s.tx.ExecContext(context.Background(), "ROLLBACK TO SAVEPOINT "+s.savepoint)
	return err
}

func (s *savepointTx) Close() error {
	return nil
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
	// QueryHook, when set, is called after every SQL statement with the query text,
	// bound arguments, and execution duration. Use this to feed the Astra Cockpit
	// SQL Timeline without importing the core package.
	QueryHook QueryHook
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

// ─── dashboardConn — SQL Timeline hook ────────────────────────────────────────

// dashboardConn wraps a Connection and fires a QueryHook after every statement.
// It is inserted into the connection chain by Open when cfg.QueryHook != nil.
// This keeps the hot-path minimal: one time.Now() + defer + non-nil func call.
type dashboardConn struct {
	inner Connection
	hook  QueryHook
}

func (d *dashboardConn) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := d.inner.Exec(ctx, sqlStr, args...)
	d.hook(sqlStr, args, time.Since(start))
	return res, err
}

func (d *dashboardConn) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	start := time.Now()
	rows, err := d.inner.Query(ctx, sqlStr, args...)
	d.hook(sqlStr, args, time.Since(start))
	return rows, err
}

func (d *dashboardConn) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	start := time.Now()
	row := d.inner.QueryRow(ctx, sqlStr, args...)
	d.hook(sqlStr, args, time.Since(start))
	return row
}

func (d *dashboardConn) Begin(ctx context.Context) (Transaction, error) {
	return d.inner.Begin(ctx)
}

func (d *dashboardConn) Close() error {
	return d.inner.Close()
}

