package orm

import (
	"context"
)

// Repository[T] provides common CRUD and query operations for a model type T.
// It is a thin ergonomic wrapper over QueryBuilder[T] that stores a *DB reference
// and removes the need to pass db on every call site.
//
// Example usage:
//
//	type UserController struct {
//	    users *orm.Repository[User]
//	}
//
//	func NewUserController(db *orm.DB) *UserController {
//	    return &UserController{users: orm.NewRepository[User](db)}
//	}
//
//	func (c *UserController) Show(ctx context.Context, id uint) (*User, error) {
//	    return c.users.Find(ctx, id)
//	}
type Repository[T any] struct {
	db *DB
}

// NewRepository creates a Repository for model type T.
func NewRepository[T any](db *DB) *Repository[T] {
	return &Repository[T]{db: db}
}

// Query returns a fresh QueryBuilder[T] for composing complex queries.
func (r *Repository[T]) Query() *QueryBuilder[T] {
	return NewQueryBuilder[T](r.db)
}

// All returns all rows for the model (no soft-deleted).
func (r *Repository[T]) All(ctx context.Context) ([]T, error) {
	return r.Query().Get(ctx)
}

// Find retrieves a record by primary key. Returns nil, sql.ErrNoRows if not found.
func (r *Repository[T]) Find(ctx context.Context, id uint) (*T, error) {
	return r.Query().Find(id, ctx)
}

// FindBy retrieves the first record where column equals value.
func (r *Repository[T]) FindBy(ctx context.Context, column string, value any) (*T, error) {
	return r.Query().FindBy(column, value, ctx)
}

// Create inserts model and returns the persisted copy with generated ID.
func (r *Repository[T]) Create(ctx context.Context, model T) (*T, error) {
	return r.Query().Create(model, ctx)
}

// Save updates a model (all non-PK columns). model.ID must be set.
func (r *Repository[T]) Save(ctx context.Context, model *T) error {
	return r.Query().Save(model, ctx)
}

// Delete soft-deletes (or hard-deletes if soft delete is not configured).
func (r *Repository[T]) Delete(ctx context.Context, id uint) error {
	return r.Query().Where("id", "=", id).Delete(ctx)
}

// First returns the first record matching the given column/value pair.
func (r *Repository[T]) First(ctx context.Context) (*T, error) {
	return r.Query().First(ctx)
}

// Paginate returns a page of results. page is 1-indexed.
func (r *Repository[T]) Paginate(ctx context.Context, page, perPage int) (*PaginationResult[T], error) {
	return r.Query().Paginate(page, perPage, ctx)
}

// Exists returns true if any record matches where column = value.
func (r *Repository[T]) Exists(ctx context.Context, column string, value any) (bool, error) {
	return r.Query().Where(column, "=", value).Exists(ctx)
}

// Count returns the total number of (non-deleted) records.
func (r *Repository[T]) Count(ctx context.Context) (int64, error) {
	return r.Query().Count(ctx)
}

// WithDB returns a new Repository using a different DB (e.g., a transaction).
//
//	txDB := db.WithTx(tx)
//	users := repo.WithDB(txDB)
func (r *Repository[T]) WithDB(db *DB) *Repository[T] {
	return &Repository[T]{db: db}
}

// ensure context is used.
var _ = context.Background
