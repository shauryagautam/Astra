package schema

import (
	"fmt"
)

type Table struct {
	Name     string
	Columns  []*Column
	Indices  []Index
	Uniques  []Index
	Foreigns []*ForeignKey

	droppedColumns []string
	droppedIndices []string
	renameColumns  map[string]string
}

type Index struct {
	Columns []string
	Name    string
}

type ForeignKey struct {
	Column       string
	RelatedTable string
	RelatedCol   string
}

func (t *Table) ID() {
	t.Columns = append(t.Columns, &Column{
		Name:      "id",
		Type:      "BIGINT",
		IsPrimary: true,
		IsAuto:    true,
	})
}

func (t *Table) String(name string, length int) *Column {
	c := &Column{Name: name, Type: fmt.Sprintf("VARCHAR(%d)", length)}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) Text(name string) *Column {
	c := &Column{Name: name, Type: "TEXT"}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) Integer(name string) *Column {
	c := &Column{Name: name, Type: "INTEGER"}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) BigInteger(name string) *Column {
	c := &Column{Name: name, Type: "BIGINT"}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) Boolean(name string) *Column {
	c := &Column{Name: name, Type: "BOOLEAN"}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) Float(name string) *Column {
	c := &Column{Name: name, Type: "FLOAT"}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) Decimal(name string, precision, scale int) *Column {
	c := &Column{Name: name, Type: fmt.Sprintf("DECIMAL(%d,%d)", precision, scale)}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) Timestamp(name string) *Column {
	c := &Column{Name: name, Type: "TIMESTAMP"}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) Timestamps() {
	t.Timestamp("created_at").NotNull()
	t.Timestamp("updated_at").NotNull()
}

func (t *Table) SoftDeletes() {
	t.Timestamp("deleted_at").Nullable()
}

func (t *Table) AddColumn(name, colType string) *Column {
	c := &Column{Name: name, Type: colType}
	t.Columns = append(t.Columns, c)
	return c
}

func (t *Table) DropColumn(name string) {
	t.droppedColumns = append(t.droppedColumns, name)
}

func (t *Table) RenameColumn(old, new string) {
	if t.renameColumns == nil {
		t.renameColumns = make(map[string]string)
	}
	t.renameColumns[old] = new
}

func (t *Table) DropIndex(name string) {
	t.droppedIndices = append(t.droppedIndices, name)
}

func (t *Table) AddIndex(columns ...string) {
	t.Indices = append(t.Indices, Index{Columns: columns})
}

func (t *Table) AddUniqueIndex(columns ...string) {
	t.Uniques = append(t.Uniques, Index{Columns: columns})
}

func (t *Table) Foreign(column string) *ForeignKey {
	fk := &ForeignKey{Column: column}
	t.Foreigns = append(t.Foreigns, fk)
	return fk
}

func (fk *ForeignKey) References(table, column string) {
	fk.RelatedTable = table
	fk.RelatedCol = column
}
