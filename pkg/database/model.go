package database

import "time"

// Model is a base struct for all Astra models, providing ID and Timestamps.
type Model struct {
	ID        uint       `orm:"primary_key;auto_increment" json:"id" db:"id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `orm:"soft_delete" json:"deleted_at,omitempty" db:"deleted_at"`
}

// Relation is the base for all relationship wrappers.
type Relation[T any] struct {
	loaded bool
}

// HasOne represents a 1-to-1 relationship.
type HasOne[T any] struct {
	Relation[T]
	item *T
}

func (r *HasOne[T]) Get() *T { return r.item }

// HasMany represents a 1-to-N relationship.
type HasMany[T any] struct {
	Relation[T]
	items []T
}

func (r *HasMany[T]) All() []T { return r.items }

// BelongsTo represents the inverse of a HasOne/HasMany.
type BelongsTo[T any] struct {
	Relation[T]
	item *T
}

func (r *BelongsTo[T]) Get() *T { return r.item }

// ManyToMany represents a N-to-N relationship via a pivot table.
type ManyToMany[T any] struct {
	Relation[T]
	items []T
}

func (r *ManyToMany[T]) All() []T { return r.items }

// MorphTo represents a polymorphic relation.
type MorphTo struct {
	Relation[any]
	item any
}

// MorphMany represents a polymorphic one-to-many relation.
type MorphMany[T any] struct {
	Relation[T]
	items []T
}
