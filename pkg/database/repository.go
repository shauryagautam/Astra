package database

import (
	"context"
)

// Repository defines the standard interface for Astra repositories.
type Repository[T any] interface {
	FindByID(ctx context.Context, id any) (*T, error)
	FindAll(ctx context.Context, page, perPage int) ([]T, error)
	Create(ctx context.Context, model *T) (*T, error)
	Update(ctx context.Context, model *T) error
	Delete(ctx context.Context, id any) error
	Count(ctx context.Context) (int64, error)
}

// BaseRepository provides a generic implementation of the Repository interface.
type BaseRepository[T any] struct {
	DB *DB
}

// NewBaseRepository creates a new BaseRepository for the given model type.
func NewBaseRepository[T any](db *DB) *BaseRepository[T] {
	return &BaseRepository[T]{DB: db}
}

// FindByID returns a single record by its primary key.
func (r *BaseRepository[T]) FindByID(ctx context.Context, id any) (*T, error) {
	return Query[T](r.DB, ctx).FindByID(id)
}

// FindAll returns a slice of records, optionally paginated.
func (r *BaseRepository[T]) FindAll(ctx context.Context, page, perPage int) ([]T, error) {
	q := Query[T](r.DB, ctx)
	if perPage > 0 {
		q = q.Limit(perPage).Offset((page - 1) * perPage)
	}
	return q.Get()
}

// Create inserts a new record into the database.
func (r *BaseRepository[T]) Create(ctx context.Context, model *T) (*T, error) {
	return Query[T](r.DB, ctx).Create(model)
}

// Update saves changes to an existing record.
func (r *BaseRepository[T]) Update(ctx context.Context, model *T) error {
	return Query[T](r.DB, ctx).Save(model)
}

// Delete removes a record by its primary key.
func (r *BaseRepository[T]) Delete(ctx context.Context, id any) error {
	// We use the PK column name from metadata to ensure correct deletion.
	q := Query[T](r.DB, ctx)
	return q.Where(q.meta.PK.ColumnName, "=", id).Delete()
}

// Count returns the total number of records.
func (r *BaseRepository[T]) Count(ctx context.Context) (int64, error) {
	return Query[T](r.DB, ctx).Count()
}
