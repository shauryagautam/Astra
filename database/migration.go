// Package database provides migration runner and schema builder.
// Mirrors Astra's database/migrations system.
package database

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"gorm.io/gorm"
)

// Migration represents a single database migration.
// Mirrors Astra migration files.
type Migration struct {
	// Name is the unique migration identifier (e.g., "20240101120000_create_users_table").
	Name string

	// Up runs the migration forward.
	Up func(db *gorm.DB) error

	// Down rolls back the migration.
	Down func(db *gorm.DB) error
}

// MigrationRecord tracks which migrations have been applied.
type MigrationRecord struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex;not null"`
	Batch     int       `gorm:"not null"`
	AppliedAt time.Time `gorm:"not null"`
}

func (MigrationRecord) TableName() string {
	return "astra_migrations"
}

// MigrationRunner manages database migrations.
// Mirrors Astra's migration:run and migration:rollback commands.
type MigrationRunner struct {
	db         *gorm.DB
	migrations []Migration
	logger     *log.Logger
}

// NewMigrationRunner creates a new migration runner.
func NewMigrationRunner(db *gorm.DB) *MigrationRunner {
	return &MigrationRunner{
		db:         db,
		migrations: make([]Migration, 0),
		logger:     log.New(os.Stdout, "[astra:migration] ", log.LstdFlags),
	}
}

// Add registers one or more migrations.
func (r *MigrationRunner) Add(migrations ...Migration) {
	r.migrations = append(r.migrations, migrations...)
}

// ensureMigrationsTable creates the migrations tracking table if not present.
func (r *MigrationRunner) ensureMigrationsTable() error {
	return r.db.AutoMigrate(&MigrationRecord{})
}

// Run executes all pending migrations.
// Mirrors: node ace migration:run
func (r *MigrationRunner) Run() error {
	if err := r.ensureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get already applied migrations
	applied := make(map[string]bool)
	var records []MigrationRecord
	r.db.Find(&records)
	for _, rec := range records {
		applied[rec.Name] = true
	}

	// Determine next batch number
	var maxBatch int
	r.db.Model(&MigrationRecord{}).Select("COALESCE(MAX(batch), 0)").Scan(&maxBatch)
	nextBatch := maxBatch + 1

	// Sort migrations by name for deterministic order
	sort.Slice(r.migrations, func(i, j int) bool {
		return r.migrations[i].Name < r.migrations[j].Name
	})

	// Run pending migrations
	pending := 0
	for _, migration := range r.migrations {
		if applied[migration.Name] {
			continue
		}

		r.logger.Printf("▶ Running migration: %s", migration.Name)
		if err := migration.Up(r.db); err != nil {
			return fmt.Errorf("migration '%s' failed: %w", migration.Name, err)
		}

		// Record the migration
		r.db.Create(&MigrationRecord{
			Name:      migration.Name,
			Batch:     nextBatch,
			AppliedAt: time.Now(),
		})

		r.logger.Printf("✅ Completed: %s", migration.Name)
		pending++
	}

	if pending == 0 {
		r.logger.Println("Nothing to migrate — already up to date")
	} else {
		r.logger.Printf("✅ Ran %d migration(s) in batch %d", pending, nextBatch)
	}

	return nil
}

// Rollback rolls back the last batch of migrations.
// Mirrors: node ace migration:rollback
func (r *MigrationRunner) Rollback() error {
	return r.RollbackBatch(0)
}

// RollbackBatch rolls back a specific batch (0 = latest).
func (r *MigrationRunner) RollbackBatch(batch int) error {
	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}

	if batch == 0 {
		// Get latest batch
		r.db.Model(&MigrationRecord{}).Select("COALESCE(MAX(batch), 0)").Scan(&batch)
		if batch == 0 {
			r.logger.Println("Nothing to rollback")
			return nil
		}
	}

	// Get migrations in this batch (reverse order)
	var records []MigrationRecord
	r.db.Where("batch = ?", batch).Order("id DESC").Find(&records)

	if len(records) == 0 {
		r.logger.Printf("No migrations found in batch %d", batch)
		return nil
	}

	// Build a name→migration map
	migrationMap := make(map[string]Migration)
	for _, m := range r.migrations {
		migrationMap[m.Name] = m
	}

	for _, record := range records {
		migration, ok := migrationMap[record.Name]
		if !ok {
			r.logger.Printf("⚠️  Migration '%s' not found in registry, skipping", record.Name)
			continue
		}

		r.logger.Printf("◀ Rolling back: %s", record.Name)
		if err := migration.Down(r.db); err != nil {
			return fmt.Errorf("rollback of '%s' failed: %w", record.Name, err)
		}

		r.db.Delete(&record)
		r.logger.Printf("✅ Rolled back: %s", record.Name)
	}

	r.logger.Printf("✅ Rolled back batch %d (%d migrations)", batch, len(records))
	return nil
}

// Reset rolls back ALL migrations.
// Mirrors: node ace migration:reset
func (r *MigrationRunner) Reset() error {
	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}

	var maxBatch int
	r.db.Model(&MigrationRecord{}).Select("COALESCE(MAX(batch), 0)").Scan(&maxBatch)

	for batch := maxBatch; batch >= 1; batch-- {
		if err := r.RollbackBatch(batch); err != nil {
			return err
		}
	}

	return nil
}

// Refresh resets and re-runs all migrations.
// Mirrors: node ace migration:refresh
func (r *MigrationRunner) Refresh() error {
	if err := r.Reset(); err != nil {
		return err
	}
	return r.Run()
}

// Status returns the current migration status.
// Mirrors: node ace migration:status
func (r *MigrationRunner) Status() ([]MigrationStatus, error) {
	if err := r.ensureMigrationsTable(); err != nil {
		return nil, err
	}

	applied := make(map[string]MigrationRecord)
	var records []MigrationRecord
	r.db.Find(&records)
	for _, rec := range records {
		applied[rec.Name] = rec
	}

	var statuses []MigrationStatus
	for _, m := range r.migrations {
		status := MigrationStatus{Name: m.Name, Status: "Pending"}
		if rec, ok := applied[m.Name]; ok {
			status.Status = "Ran"
			status.Batch = rec.Batch
			status.AppliedAt = rec.AppliedAt
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// MigrationStatus represents the status of a single migration.
type MigrationStatus struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Batch     int       `json:"batch"`
	AppliedAt time.Time `json:"applied_at"`
}

// ══════════════════════════════════════════════════════════════════════
// Schema Builder
//
// Provides helpers for common schema operations inside migration Up/Down.
// Mirrors Astra's this.schema.createTable() etc.
// ══════════════════════════════════════════════════════════════════════

// Schema provides schema operations for use in migrations.
type Schema struct {
	DB *gorm.DB
}

// NewSchema creates a new Schema helper.
func NewSchema(db *gorm.DB) *Schema {
	return &Schema{DB: db}
}

// CreateTable creates a table using GORM's AutoMigrate for the provided model.
func (s *Schema) CreateTable(models ...any) error {
	return s.DB.AutoMigrate(models...)
}

// DropTable drops one or more tables.
func (s *Schema) DropTable(tables ...string) error {
	for _, table := range tables {
		if err := s.DB.Migrator().DropTable(table); err != nil {
			return err
		}
	}
	return nil
}

// DropTableIfExists drops a table if it exists.
func (s *Schema) DropTableIfExists(table string) error {
	if s.DB.Migrator().HasTable(table) {
		return s.DB.Migrator().DropTable(table)
	}
	return nil
}

// HasTable checks if a table exists.
func (s *Schema) HasTable(table string) bool {
	return s.DB.Migrator().HasTable(table)
}

// HasColumn checks if a column exists in a table.
func (s *Schema) HasColumn(table string, column string) bool {
	return s.DB.Migrator().HasColumn(table, column)
}

// AddColumn adds a column via raw SQL.
func (s *Schema) AddColumn(table string, column string, dataType string) error {
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, dataType)
	return s.DB.Exec(sql).Error
}

// DropColumn drops a column from a table.
func (s *Schema) DropColumn(table string, column string) error {
	sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)
	return s.DB.Exec(sql).Error
}

// RenameColumn renames a column.
func (s *Schema) RenameColumn(table string, from string, to string) error {
	sql := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, from, to)
	return s.DB.Exec(sql).Error
}

// RenameTable renames a table.
func (s *Schema) RenameTable(from string, to string) error {
	sql := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", from, to)
	return s.DB.Exec(sql).Error
}

// AddIndex adds an index on columns.
func (s *Schema) AddIndex(table string, name string, columns ...string) error {
	cols := ""
	for i, col := range columns {
		if i > 0 {
			cols += ", "
		}
		cols += col
	}
	sql := fmt.Sprintf("CREATE INDEX %s ON %s (%s)", name, table, cols)
	return s.DB.Exec(sql).Error
}

// AddUniqueIndex adds a unique index.
func (s *Schema) AddUniqueIndex(table string, name string, columns ...string) error {
	cols := ""
	for i, col := range columns {
		if i > 0 {
			cols += ", "
		}
		cols += col
	}
	sql := fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", name, table, cols)
	return s.DB.Exec(sql).Error
}

// DropIndex drops an index.
func (s *Schema) DropIndex(table string, name string) error {
	sql := fmt.Sprintf("DROP INDEX %s", name)
	return s.DB.Exec(sql).Error
}

// Raw executes raw SQL.
func (s *Schema) Raw(sql string, args ...any) error {
	return s.DB.Exec(sql, args...).Error
}
