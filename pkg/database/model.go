package database

import "time"

// Model is an explicit embeddable set of common persistence fields. Embedding
// it is optional; a model may define its own fields and metadata instead.
//
// The orm tag controls persistence metadata:
//   - primary_key marks the identifier.
//   - auto_increment excludes a value on insert.
//   - soft_delete enables Delete as an UPDATE that sets DeletedAt.
//   - guarded excludes a field from Create and map-based Update.
//   - column:name overrides the Go-name-to-column mapping.
type Model struct {
	ID        uint       `orm:"primary_key;auto_increment" json:"id" db:"id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `orm:"soft_delete" json:"deleted_at,omitempty" db:"deleted_at"`
}

// Relation is the base for all relationship wrappers.
type Relation[T any] struct {
	Loaded bool `json:"-"`
}

// HasOne represents a 1-to-1 relationship.
type HasOne[T any] struct {
	Relation[T]
	Item *T
}

func (r *HasOne[T]) Get() *T { return r.Item }

// HasMany represents a 1-to-N relationship.
type HasMany[T any] struct {
	Relation[T]
	Items []T
}

func (r *HasMany[T]) All() []T { return r.Items }

// BelongsTo represents the inverse of a HasOne/HasMany.
type BelongsTo[T any] struct {
	Relation[T]
	Item *T
}

func (r *BelongsTo[T]) Get() *T { return r.Item }

// ManyToMany represents a N-to-N relationship via a pivot table.
type ManyToMany[T any] struct {
	Relation[T]
	Items []T
}

func (r *ManyToMany[T]) All() []T { return r.Items }

// MorphTo represents a polymorphic relation.
type MorphTo struct {
	Relation[any]
	Item any
}

func (r *MorphTo) Get() any { return r.Item }

// MorphMany represents a polymorphic one-to-many relation.
type MorphMany[T any] struct {
	Relation[T]
	Items []T
}
