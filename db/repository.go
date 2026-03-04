package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides a generic CRUD interface for a specific table.
type Repository[T any] struct {
	DB    *pgxpool.Pool
	Table string
}

// NewRepository creates a new generic repository.
func NewRepository[T any](db *pgxpool.Pool, table string) *Repository[T] {
	return &Repository[T]{
		DB:    db,
		Table: table,
	}
}

// Query returns a new QueryBuilder for this repository.
func (r *Repository[T]) Query() *QueryBuilder[T] {
	return NewQueryBuilder[T](r.DB, r.Table)
}

// Find retrieves a record by its ID.
func (r *Repository[T]) Find(ctx context.Context, id any) (*T, error) {
	return r.Query().Where("id = ?", id).First(ctx)
}

// FindBy retrieves a record by a specific column and value.
func (r *Repository[T]) FindBy(ctx context.Context, col string, val any) (*T, error) {
	return r.Query().Where(col+" = ?", val).First(ctx)
}

// All retrieves all records.
func (r *Repository[T]) All(ctx context.Context) ([]T, error) {
	return r.Query().All(ctx)
}

// Create inserts a new record and returns it.
func (r *Repository[T]) Create(ctx context.Context, data map[string]any) (*T, error) {
	var m T
	if hook, ok := any(&m).(BeforeCreateHook); ok {
		if err := hook.BeforeCreate(ctx); err != nil {
			return nil, err
		}
	}

	res, err := r.Query().Create(ctx, data)
	if err != nil {
		return nil, err
	}

	if hook, ok := any(res).(AfterCreateHook); ok {
		if err := hook.AfterCreate(ctx); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// Update modifies an existing record by ID and returns the updated record.
func (r *Repository[T]) Update(ctx context.Context, id any, data map[string]any) (*T, error) {
	var m T
	if hook, ok := any(&m).(BeforeUpdateHook); ok {
		if err := hook.BeforeUpdate(ctx); err != nil {
			return nil, err
		}
	}

	res, err := r.Query().Where("id = ?", id).Update(ctx, data)
	if err != nil {
		return nil, err
	}

	if hook, ok := any(res).(AfterUpdateHook); ok {
		if err := hook.AfterUpdate(ctx); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// Delete removes a record by ID.
func (r *Repository[T]) Delete(ctx context.Context, id any) error {
	var m T
	if hook, ok := any(&m).(BeforeDeleteHook); ok {
		if err := hook.BeforeDelete(ctx); err != nil {
			return err
		}
	}

	err := r.Query().Where("id = ?", id).Delete(ctx)
	if err != nil {
		return err
	}

	if hook, ok := any(&m).(AfterDeleteHook); ok {
		if err := hook.AfterDelete(ctx); err != nil {
			return err
		}
	}

	return nil
}
