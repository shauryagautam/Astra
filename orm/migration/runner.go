package migration

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/astraframework/astra/orm"
	"github.com/astraframework/astra/orm/schema"
)

type Runner struct {
	db       *orm.DB
	dir      string
	registry map[string]Migration
}

func NewRunner(db *orm.DB, dir string) *Runner {
	return &Runner{
		db:       db,
		dir:      dir,
		registry: make(map[string]Migration),
	}
}

func (r *Runner) DB() *orm.DB {
	return r.db
}

func (r *Runner) Register(name string, m Migration) {
	r.registry[name] = m
}

func (r *Runner) Up() error {
	// Acquire distributed lock
	lockID := int64(123456789) // Constant for migration lock
	lockSQL := r.db.Dialect().AdvisoryLock(lockID)
	if lockSQL != "" {
		if _, err := r.db.Exec(context.Background(), lockSQL); err != nil {
			return fmt.Errorf("migration: failed to acquire lock: %w", err)
		}
		defer func() {
			unlockSQL := r.db.Dialect().AdvisoryUnlock(lockID)
			if unlockSQL != "" {
				_, _ = r.db.Exec(context.Background(), unlockSQL)
			}
		}()
	}

	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}

	applied, err := r.getAppliedMigrations()
	if err != nil {
		return err
	}

	var pending []string
	for name := range r.registry {
		if !contains(applied, name) {
			pending = append(pending, name)
		}
	}
	sort.Strings(pending)

	if len(pending) == 0 {
		fmt.Println("Nothing to migrate.")
		return nil
	}

	batch, err := r.getNextBatch()
	if err != nil {
		return err
	}

	for _, name := range pending {
		fmt.Printf("Migrating: %s\n", name)
		m := r.registry[name]
		if err := m.Up(r.db.Schema()); err != nil {
			return fmt.Errorf("failed to migrate %s: %v", name, err)
		}
		if err := r.markAsApplied(name, batch); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) Rollback() error {
	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}

	lastBatch, err := r.getLastBatchNumber()
	if err != nil {
		return err
	}

	if lastBatch == 0 {
		fmt.Println("Nothing to rollback.")
		return nil
	}

	migrations, err := r.getMigrationsInBatch(lastBatch)
	if err != nil {
		return err
	}

	// Rollback in reverse order
	sort.Sort(sort.Reverse(sort.StringSlice(migrations)))

	for _, name := range migrations {
		fmt.Printf("Rolling back: %s\n", name)
		m, ok := r.registry[name]
		if !ok {
			return fmt.Errorf("migration %s not found in registry", name)
		}

		if err := m.Down(r.db.Schema()); err != nil {
			return fmt.Errorf("failed to rollback %s: %v", name, err)
		}

		if err := r.removeApplied(name); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) Status() error {
	if err := r.ensureMigrationsTable(); err != nil {
		return err
	}

	appliedMap, err := r.getAppliedMigrationsWithBatch()
	if err != nil {
		return err
	}

	var allMigrations []string
	for name := range r.registry {
		allMigrations = append(allMigrations, name)
	}
	sort.Strings(allMigrations)

	fmt.Printf("%-4s | %-40s | %-5s | %-20s\n", "Ran?", "Migration", "Batch", "Applied At")
	fmt.Printf("%-4s | %-40s | %-5s | %-20s\n", "----", "----------------------------------------", "-----", "--------------------")

	for _, name := range allMigrations {
		status := "No"
		batchStr := ""
		appliedAt := ""
		if m, ok := appliedMap[name]; ok {
			status = "Yes"
			batchStr = fmt.Sprintf("%d", m.Batch)
			appliedAt = m.AppliedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("%-4s | %-40s | %-5s | %-20s\n", status, name, batchStr, appliedAt)
	}

	return nil
}

type appliedInfo struct {
	Batch     int
	AppliedAt time.Time
}

func (r *Runner) getAppliedMigrationsWithBatch() (map[string]appliedInfo, error) {
	rows, err := r.db.Raw("SELECT migration, batch, applied_at FROM astra_migrations").Rows()
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	result := make(map[string]appliedInfo)
	for rows.Next() {
		var name string
		var batch int
		var appliedAt time.Time
		if err := rows.Scan(&name, &batch, &appliedAt); err == nil {
			result[name] = appliedInfo{Batch: batch, AppliedAt: appliedAt}
		}
	}
	return result, nil
}

func (r *Runner) getLastBatchNumber() (int, error) {
	rows, err := r.db.Raw("SELECT COALESCE(MAX(batch), 0) FROM astra_migrations").Rows()
	if err != nil {
		return 0, nil
	}
	defer rows.Close()

	var batch int
	if rows.Next() {
		_ = rows.Scan(&batch)
	}
	return batch, nil
}

func (r *Runner) getMigrationsInBatch(batch int) ([]string, error) {
	rows, err := r.db.Raw("SELECT migration FROM astra_migrations WHERE batch = ?", batch).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			result = append(result, name)
		}
	}
	return result, nil
}

func (r *Runner) removeApplied(name string) error {
	_, err := r.db.Exec(context.Background(), "DELETE FROM astra_migrations WHERE migration = ?", name)
	return err
}

func (r *Runner) ensureMigrationsTable() error {
	return r.db.Schema().CreateTableIfNotExists("astra_migrations", func(t *schema.Table) {
		t.ID()
		t.String("migration", 255).Unique()
		t.Integer("batch")
		t.Timestamp("applied_at")
	})
}

func (r *Runner) getAppliedMigrations() ([]string, error) {
	// Use Raw to avoid circular dependency if using MigrationModel
	rows, err := r.db.Raw("SELECT migration FROM astra_migrations").Rows()
	if err != nil {
		// Table might not exist yet
		return nil, nil
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			result = append(result, name)
		}
	}
	return result, nil
}

func (r *Runner) getNextBatch() (int, error) {
	rows, err := r.db.Raw("SELECT COALESCE(MAX(batch), 0) FROM astra_migrations").Rows()
	if err != nil {
		return 1, nil
	}
	defer rows.Close()

	var batch int
	if rows.Next() {
		_ = rows.Scan(&batch)
	}
	return batch + 1, nil
}

func (r *Runner) markAsApplied(name string, batch int) error {
	_, err := r.db.Exec(context.Background(),
		"INSERT INTO astra_migrations (migration, batch, applied_at) VALUES (?, ?, ?)", name, batch, time.Now())
	return err
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
