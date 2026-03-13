package migration

import (
	"time"

	"github.com/astraframework/astra/orm/schema"
)

// Migration interface for defining database changes
type Migration interface {
	Up(schema *schema.Builder) error
	Down(schema *schema.Builder) error
}

// SimpleMigration allows defining migrations using anonymous functions
type SimpleMigration struct {
	Name   string
	UpFn   func(*schema.Builder) error
	DownFn func(*schema.Builder) error
}

func (m *SimpleMigration) Up(s *schema.Builder) error {
	if m.UpFn != nil {
		return m.UpFn(s)
	}
	return nil
}

func (m *SimpleMigration) Down(s *schema.Builder) error {
	if m.DownFn != nil {
		return m.DownFn(s)
	}
	return nil
}

// Metadata for tracking migrations in the database
type MigrationModel struct {
	ID        uint      `orm:"primaryKey;autoIncrement"`
	Migration string    `orm:"not_null;unique"`
	Batch     int       `orm:"not_null"`
	AppliedAt time.Time `orm:"not_null"`
}

func (m MigrationModel) TableName() string {
	return "astra_migrations"
}
