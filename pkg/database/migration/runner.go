package migration // Astra Migration Runner

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shauryagautam/Astra/pkg/database"
)

// MigrationRecord represents a migration that has been applied to the database.
type MigrationRecord struct {
	ID       int
	Name     string
	Batch    int
	RunAt    time.Time
	Checksum string
}

// Runner handles running and rolling back migrations.
type Runner struct {
	db      *sql.DB
	dialect database.Dialect
	dir     string
	fs      fs.FS
}

// NewRunner creates a new migration runner.
func NewRunner(db *sql.DB, dialect database.Dialect, dir string, fileSystem fs.FS) *Runner {
	if fileSystem == nil {
		fileSystem = osFS{dir: dir}
	}
	return &Runner{
		db:      db,
		dialect: dialect,
		dir:     dir,
		fs:      fileSystem,
	}
}

// Setup ensures the migrations table exists with all required columns.
func (r *Runner) Setup(ctx context.Context) error {
	var query string
	switch r.dialect.Name() {
	case "postgres", "neon":
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id         SERIAL PRIMARY KEY,
				version    VARCHAR(255) NOT NULL UNIQUE,
				batch      INT NOT NULL DEFAULT 1,
				run_at     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				checksum   VARCHAR(64) NOT NULL DEFAULT ''
			)
		`
	case "mysql":
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
				version    VARCHAR(255) NOT NULL UNIQUE,
				batch      INT NOT NULL DEFAULT 1,
				run_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				checksum   VARCHAR(64) NOT NULL DEFAULT ''
			)
		`
	default: // sqlite / default
		query = `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				version    VARCHAR(255) NOT NULL UNIQUE,
				batch      INT NOT NULL DEFAULT 1,
				run_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
				checksum   VARCHAR(64) NOT NULL DEFAULT ''
			)
		`
	}

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to setup migrations table: %w", err)
	}

	// Add columns to existing tables that may be missing them (idempotent upgrades)
	runAtDef := "TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP"
	if r.dialect.Name() == "mysql" {
		runAtDef = "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
	} else if r.dialect.Name() == "sqlite" {
		runAtDef = "DATETIME DEFAULT CURRENT_TIMESTAMP"
	}

	for _, col := range []struct{ name, def string }{
		{"batch", "INT NOT NULL DEFAULT 1"},
		{"checksum", "VARCHAR(64) NOT NULL DEFAULT ''"},
		{"run_at", runAtDef},
	} {
		var alterQuery string
		if r.dialect.Name() == "mysql" {
			// MySQL does not support ADD COLUMN IF NOT EXISTS.
			// We just attempt the ADD COLUMN; we'll ignore the error if it already exists.
			alterQuery = fmt.Sprintf("ALTER TABLE schema_migrations ADD COLUMN %s %s", col.name, col.def)
		} else {
			alterQuery = fmt.Sprintf("ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS %s %s", col.name, col.def)
		}
		_, _ = r.db.ExecContext(ctx, alterQuery)
	}
	return nil
}

// acquireLock acquires a database-specific advisory lock to prevent concurrent migrations.
// Returns a release function that must be deferred.
func (r *Runner) acquireLock(ctx context.Context) (func(), error) {
	const lockID = 999_888_777 // arbitrary consistent lock ID for migrations

	switch r.dialect.Name() {
	case "postgres", "neon":
		var got bool
		if err := r.db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&got); err != nil {
			return nil, fmt.Errorf("failed to acquire migration lock: %w", err)
		}
		if !got {
			return nil, fmt.Errorf("another migration is already running (advisory lock held)")
		}
		release := func() {
			_, _ = r.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID)
		}
		return release, nil

	case "mysql":
		var got sql.NullInt64
		lockName := fmt.Sprintf("astra_migration_%d", lockID)
		if err := r.db.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", lockName).Scan(&got); err != nil {
			return nil, fmt.Errorf("failed to acquire migration lock: %w", err)
		}
		if !got.Valid || got.Int64 != 1 {
			return nil, fmt.Errorf("another migration is already running (GET_LOCK timed out or failed)")
		}
		release := func() {
			_, _ = r.db.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", lockName)
		}
		return release, nil

	default:
		// sqlite or other dialects: skip advisory locking
		return func() {}, nil
	}
}

// Run executes all pending migrations in order.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.Setup(ctx); err != nil {
		return err
	}

	release, err := r.acquireLock(ctx)
	if err != nil {
		return err
	}
	defer release()

	applied, err := r.getApplied(ctx)
	if err != nil {
		return err
	}

	files, err := fs.ReadDir(r.fs, ".")
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	var pending []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			version := strings.TrimSuffix(f.Name(), ".sql")
			if rec, ok := applied[version]; ok {
				// Check for checksum mismatch (tampered migration)
				content, _ := fs.ReadFile(r.fs, f.Name())
				checksum := computeChecksum(string(content))
				if rec.Checksum != "" && rec.Checksum != checksum {
					return fmt.Errorf("migration %s was modified after being applied (checksum mismatch)", version)
				}
			} else {
				pending = append(pending, f.Name())
			}
		}
	}

	sort.Strings(pending)

	// Determine next batch number
	nextBatch := 1
	var maxBatch int
	if err := r.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(batch),0) FROM schema_migrations").Scan(&maxBatch); err == nil {
		nextBatch = maxBatch + 1
	}

	for _, file := range pending {
		content, err := fs.ReadFile(r.fs, file)
		if err != nil {
			return err
		}

		upSQL, _ := parseMigration(string(content))
		if upSQL == "" {
			continue
		}

		checksum := computeChecksum(string(content))
		version := strings.TrimSuffix(file, ".sql")

		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				// Ignore rollback error
			}
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}

		insertQuery := fmt.Sprintf(
			"INSERT INTO schema_migrations (version, batch, checksum) VALUES (%s, %s, %s)",
			r.dialect.Placeholder(1),
			r.dialect.Placeholder(2),
			r.dialect.Placeholder(3),
		)
		if _, err := tx.ExecContext(ctx, insertQuery, version, nextBatch, checksum); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				// Ignore rollback error
			}
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		fmt.Printf("  ✓ Applied  [batch %d] %s\n", nextBatch, file)
	}

	if len(pending) == 0 {
		fmt.Println("  Nothing to migrate.")
	}
	return nil
}

// Status returns all applied migration records and a list of pending file names.
func (r *Runner) Status(ctx context.Context) (applied []MigrationRecord, pending []string, err error) {
	if err = r.Setup(ctx); err != nil {
		return
	}

	appliedMap, err := r.getApplied(ctx)
	if err != nil {
		return
	}

	files, err := fs.ReadDir(r.fs, ".")
	if err != nil {
		return
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			version := strings.TrimSuffix(f.Name(), ".sql")
			if rec, ok := appliedMap[version]; ok {
				applied = append(applied, rec)
			} else {
				pending = append(pending, f.Name())
			}
		}
	}
	sort.Strings(pending)
	return
}

// Rollback rolls back the last batch of migrations.
func (r *Runner) Rollback(ctx context.Context) error {
	return r.RollbackN(ctx, 1)
}

// RollbackN rolls back the last N individual migrations.
func (r *Runner) RollbackN(ctx context.Context, n int) error {
	if err := r.Setup(ctx); err != nil {
		return err
	}

	release, err := r.acquireLock(ctx)
	if err != nil {
		return err
	}
	defer release()

	selectQuery := fmt.Sprintf(
		"SELECT version FROM schema_migrations ORDER BY id DESC LIMIT %s",
		r.dialect.Placeholder(1),
	)
	rows, err := r.db.QueryContext(ctx, selectQuery, n)
	if err != nil {
		return err
	}
	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		versions = append(versions, v)
	}
	rows.Close()

	if len(versions) == 0 {
		fmt.Println("  Nothing to rollback.")
		return nil
	}

	for _, version := range versions {
		filename := version + ".sql"
		content, err := fs.ReadFile(r.fs, filename)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", filename, err)
		}

		_, downSQL := parseMigration(string(content))

		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if downSQL != "" {
			if _, err := tx.ExecContext(ctx, downSQL); err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					// Ignore rollback error
				}
				return fmt.Errorf("failed to rollback %s: %w", filename, err)
			}
		}

		deleteQuery := fmt.Sprintf(
			"DELETE FROM schema_migrations WHERE version = %s",
			r.dialect.Placeholder(1),
		)
		if _, err := tx.ExecContext(ctx, deleteQuery, version); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				// Ignore rollback error
			}
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		fmt.Printf("  ✓ Rolled back %s\n", filename)
	}
	return nil
}

// Fresh drops all user tables and re-runs all migrations.
// CAUTION: destructive operation — for development use only.
func (r *Runner) Fresh(ctx context.Context) error {
	if err := r.Setup(ctx); err != nil {
		return err
	}

	release, err := r.acquireLock(ctx)
	if err != nil {
		return err
	}
	defer release()

	var query string
	switch r.dialect.Name() {
	case "postgres", "neon":
		query = `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`
	case "mysql":
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name`
	case "sqlite":
		query = `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
	default:
		return fmt.Errorf("unsupported dialect for Fresh operation: %s", r.dialect.Name())
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			// Ignore scan error
		}
		tables = append(tables, t)
	}
	rows.Close()

	if len(tables) > 0 {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE",
			strings.Join(quoteIdents(tables), ", "))
		if _, err := r.db.ExecContext(ctx, dropSQL); err != nil {
			return fmt.Errorf("failed to drop tables: %w", err)
		}
		fmt.Printf("  Dropped %d table(s)\n", len(tables))
	}

	release() // release lock before re-running (Run will re-acquire)
	return r.Run(ctx)
}

// getApplied returns a map of applied migration versions to their records.
func (r *Runner) getApplied(ctx context.Context) (map[string]MigrationRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, version, batch, run_at, checksum FROM schema_migrations ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]MigrationRecord)
	for rows.Next() {
		var rec MigrationRecord
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Batch, &rec.RunAt, &rec.Checksum); err != nil {
			return nil, err
		}
		applied[rec.Name] = rec
	}
	return applied, nil
}

func parseMigration(content string) (up string, down string) {
	parts := strings.Split(content, "-- +migrate Down")
	if len(parts) > 0 {
		up = strings.Replace(parts[0], "-- +migrate Up", "", 1)
	}
	if len(parts) > 1 {
		down = parts[1]
	}
	return strings.TrimSpace(up), strings.TrimSpace(down)
}

func computeChecksum(content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
}

func quoteIdents(names []string) []string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = `"` + strings.ReplaceAll(n, `"`, `""`) + `"`
	}
	return quoted
}

type osFS struct {
	dir string
}

func (f osFS) Open(name string) (fs.File, error) {
	name = filepath.Clean(name)
	return os.Open(filepath.Join(f.dir, name)) // #nosec G304
}

func (f osFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(filepath.Join(f.dir, name))
}
