package schema

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Dialect interface for schema generation (to avoid circular dependency)
type Dialect interface {
	QuoteIdentifier(name string) string
	AutoIncrementDDL() string
}

// Executor interface for executing schema changes
type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (sql.Result, error)
}

type Builder struct {
	Dialect Dialect
	Exec    Executor
}

func (b *Builder) CreateTable(name string, fn func(*Table)) error {
	t := &Table{Name: name}
	fn(t)

	sqlStr := b.buildCreateTableSQL(t, false)
	_, err := b.Exec.Exec(context.Background(), sqlStr)
	return err
}

func (b *Builder) CreateTableIfNotExists(name string, fn func(*Table)) error {
	t := &Table{Name: name}
	fn(t)

	sqlStr := b.buildCreateTableSQL(t, true)
	_, err := b.Exec.Exec(context.Background(), sqlStr)
	return err
}

func (b *Builder) buildCreateTableSQL(t *Table, ifNotExists bool) string {
	var sb strings.Builder
	sb.WriteString("CREATE TABLE ")
	if ifNotExists {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(b.Dialect.QuoteIdentifier(t.Name))
	sb.WriteString(" (")

	for i, col := range t.Columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(b.buildColumnSQL(col))
	}

	for _, fk := range t.Foreigns {
		sb.WriteString(", ")
		sb.WriteString(fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)",
			b.Dialect.QuoteIdentifier(fk.Column),
			b.Dialect.QuoteIdentifier(fk.RelatedTable),
			b.Dialect.QuoteIdentifier(fk.RelatedCol)))
	}

	sb.WriteString(")")
	return sb.String()
}

func (b *Builder) buildColumnSQL(c *Column) string {
	var sb strings.Builder
	sb.WriteString(b.Dialect.QuoteIdentifier(c.Name))
	sb.WriteString(" ")

	colType := c.Type
	if c.IsAuto {
		colType = b.Dialect.AutoIncrementDDL()
	}
	sb.WriteString(colType)

	if !c.IsNullable {
		sb.WriteString(" NOT NULL")
	}
	if c.IsUnique {
		sb.WriteString(" UNIQUE")
	}
	if c.IsPrimary && !c.IsAuto {
		sb.WriteString(" PRIMARY KEY")
	}
	if c.DefaultValue != nil {
		sb.WriteString(fmt.Sprintf(" DEFAULT %v", c.DefaultValue))
	}

	return sb.String()
}

func (b *Builder) DropTable(name string) error {
	sql := fmt.Sprintf("DROP TABLE %s", b.Dialect.QuoteIdentifier(name))
	_, err := b.Exec.Exec(context.Background(), sql)
	return err
}

func (b *Builder) DropTableIfExists(name string) error {
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s", b.Dialect.QuoteIdentifier(name))
	_, err := b.Exec.Exec(context.Background(), sql)
	return err
}

func (b *Builder) AlterTable(name string, fn func(*Table)) error {
	t := &Table{Name: name}
	fn(t)

	for _, col := range t.Columns {
		sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s",
			b.Dialect.QuoteIdentifier(t.Name),
			b.buildColumnSQL(col))
		if _, err := b.Exec.Exec(context.Background(), sql); err != nil {
			return err
		}
	}

	for _, colName := range t.droppedColumns {
		sql := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s",
			b.Dialect.QuoteIdentifier(t.Name),
			b.Dialect.QuoteIdentifier(colName))
		if _, err := b.Exec.Exec(context.Background(), sql); err != nil {
			return err
		}
	}

	for old, new := range t.renameColumns {
		sql := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
			b.Dialect.QuoteIdentifier(t.Name),
			b.Dialect.QuoteIdentifier(old),
			b.Dialect.QuoteIdentifier(new))
		if _, err := b.Exec.Exec(context.Background(), sql); err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) HasTable(name string) (bool, error) {
	// Simple check using standard SQL if possible, but often dialect specific.
	// For now, return false as placeholder or implement basic check.
	return false, nil
}
