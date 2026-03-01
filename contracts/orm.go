package contracts

import "time"

// ModelContract defines the base interface for Lucid ORM models.
// In Astra, models extend BaseModel. In Go, we use composition
// with a generic BaseModel[T] struct that provides Active Record methods.
//
// Go Idiom Note: Astra uses class static methods (User.find(1)).
// Go doesn't have static methods on structs. We use package-level
// generic functions: model.Find[User](db, 1) — or a QueryBuilder
// pattern: User.Query().Where(...).First()
type ModelContract interface {
	// TableName returns the database table name for this model.
	TableName() string

	// PrimaryKey returns the primary key column name.
	PrimaryKey() string

	// GetID returns the primary key value.
	GetID() any

	// GetCreatedAt returns the creation timestamp.
	GetCreatedAt() time.Time

	// GetUpdatedAt returns the last update timestamp.
	GetUpdatedAt() time.Time

	// IsPersisted returns true if the model has been saved to the database.
	IsPersisted() bool
}

// QueryBuilderContract defines the chainable query builder interface.
// Mirrors Lucid's query builder with a fluent API.
//
// Usage in Astra:
//
//	const users = await User.query()
//	  .where('age', '>', 18)
//	  .orderBy('name', 'asc')
//	  .preload('posts')
//	  .paginate(1, 10)
type QueryBuilderContract interface {
	// Where adds a WHERE clause.
	// Mirrors: .where('column', 'operator', value)
	Where(query string, args ...any) QueryBuilderContract

	// WhereIn adds a WHERE IN clause.
	WhereIn(column string, values []any) QueryBuilderContract

	// WhereNull adds a WHERE IS NULL clause.
	WhereNull(column string) QueryBuilderContract

	// WhereNotNull adds a WHERE IS NOT NULL clause.
	WhereNotNull(column string) QueryBuilderContract

	// OrWhere adds an OR WHERE clause.
	OrWhere(query string, args ...any) QueryBuilderContract

	// OrderBy adds an ORDER BY clause.
	// Mirrors: .orderBy('column', 'asc'|'desc')
	OrderBy(column string, direction string) QueryBuilderContract

	// Limit sets the LIMIT.
	Limit(limit int) QueryBuilderContract

	// Offset sets the OFFSET.
	Offset(offset int) QueryBuilderContract

	// Preload eager-loads a relationship.
	// Mirrors: .preload('posts')
	Preload(relation string, args ...any) QueryBuilderContract

	// Select specifies columns to select.
	Select(columns ...string) QueryBuilderContract

	// Count returns the count of matching records.
	Count() (int64, error)

	// First returns the first matching record.
	First(dest any) error

	// All returns all matching records.
	All(dest any) error

	// Paginate returns paginated results.
	Paginate(page int, perPage int) (*PaginationResult, error)

	// Delete deletes matching records.
	Delete() error

	// Update updates matching records.
	Update(values map[string]any) error
}

// PaginationResult holds paginated query results.
// Mirrors Lucid's SimplePaginator.
type PaginationResult struct {
	Data        any   `json:"data"`
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	HasMore     bool  `json:"has_more"`
}

// HookType identifies when a model hook should fire.
type HookType string

const (
	HookBeforeCreate HookType = "before_create"
	HookAfterCreate  HookType = "after_create"
	HookBeforeSave   HookType = "before_save"
	HookAfterSave    HookType = "after_save"
	HookBeforeUpdate HookType = "before_update"
	HookAfterUpdate  HookType = "after_update"
	HookBeforeDelete HookType = "before_delete"
	HookAfterDelete  HookType = "after_delete"
	HookAfterFind    HookType = "after_find"
)

// MigrationContract defines a single database migration.
// Mirrors Astra's migration class with up/down methods.
type MigrationContract interface {
	// Up runs the migration (create table, add column, etc.).
	Up() error

	// Down rolls back the migration.
	Down() error

	// Name returns a unique identifier for this migration.
	// Convention: "YYYYMMDDHHMMSS_description"
	Name() string
}

// SchemaBuilderContract builds database schema operations.
// Mirrors Astra's Schema class used inside migrations.
type SchemaBuilderContract interface {
	// CreateTable creates a new database table.
	CreateTable(name string, callback func(table TableBuilderContract)) error

	// AlterTable modifies an existing table.
	AlterTable(name string, callback func(table TableBuilderContract)) error

	// DropTable drops a table.
	DropTable(name string) error

	// DropTableIfExists drops a table if it exists.
	DropTableIfExists(name string) error

	// RenameTable renames a table.
	RenameTable(from string, to string) error

	// HasTable checks if a table exists.
	HasTable(name string) (bool, error)

	// Raw executes raw SQL.
	Raw(sql string, args ...any) error
}

// TableBuilderContract defines table column operations inside a migration.
type TableBuilderContract interface {
	// Increments adds an auto-incrementing integer primary key.
	Increments(name string) ColumnBuilderContract

	// BigIncrements adds an auto-incrementing bigint primary key.
	BigIncrements(name string) ColumnBuilderContract

	// String adds a VARCHAR column.
	String(name string, length ...int) ColumnBuilderContract

	// Text adds a TEXT column.
	Text(name string) ColumnBuilderContract

	// Integer adds an INTEGER column.
	Integer(name string) ColumnBuilderContract

	// BigInteger adds a BIGINT column.
	BigInteger(name string) ColumnBuilderContract

	// Boolean adds a BOOLEAN column.
	Boolean(name string) ColumnBuilderContract

	// Float adds a FLOAT column.
	Float(name string) ColumnBuilderContract

	// Decimal adds a DECIMAL column.
	Decimal(name string, precision int, scale int) ColumnBuilderContract

	// DateTime adds a TIMESTAMP column.
	DateTime(name string) ColumnBuilderContract

	// Date adds a DATE column.
	Date(name string) ColumnBuilderContract

	// JSON adds a JSONB column.
	JSON(name string) ColumnBuilderContract

	// UUID adds a UUID column.
	UUID(name string) ColumnBuilderContract

	// Timestamps adds created_at and updated_at columns.
	Timestamps()

	// DropColumn drops a column.
	DropColumn(name string)

	// Index adds an index on columns.
	Index(columns ...string)

	// Unique adds a unique constraint on columns.
	Unique(columns ...string)

	// Foreign adds a foreign key constraint.
	Foreign(column string) ForeignKeyContract
}

// ColumnBuilderContract allows chaining column modifiers.
type ColumnBuilderContract interface {
	// Nullable marks the column as nullable.
	Nullable() ColumnBuilderContract

	// NotNullable marks the column as NOT NULL.
	NotNullable() ColumnBuilderContract

	// DefaultTo sets a default value.
	DefaultTo(value any) ColumnBuilderContract

	// Unique adds a unique constraint to this column.
	Unique() ColumnBuilderContract

	// Primary marks this column as the primary key.
	Primary() ColumnBuilderContract

	// References sets up a foreign key reference.
	References(column string) ForeignKeyContract
}

// ForeignKeyContract defines a foreign key constraint.
type ForeignKeyContract interface {
	// References sets the referenced column.
	References(column string) ForeignKeyContract

	// InTable sets the referenced table.
	InTable(table string) ForeignKeyContract

	// OnDelete sets the ON DELETE action.
	OnDelete(action string) ForeignKeyContract

	// OnUpdate sets the ON UPDATE action.
	OnUpdate(action string) ForeignKeyContract
}
