package db

import (
	"context"


	"gorm.io/gorm"
)

// Repository provides a generic CRUD interface for a specific model type T.
type Repository[T any] struct {
	DB *gorm.DB
}

// NewRepository creates a new generic repository wrapping a GORM connection.
func NewRepository[T any](db *gorm.DB) *Repository[T] {
	return &Repository[T]{
		DB: db,
	}
}

// Query returns the underlying *gorm.DB query instance for the model,
// allowing chainable GORM methods (Where, Preload, Joins, etc).
func (r *Repository[T]) Query(ctx context.Context) *gorm.DB {
	var model T
	return r.DB.WithContext(ctx).Model(&model)
}

// Find retrieves a record by its primary key.
func (r *Repository[T]) Find(ctx context.Context, id any) (*T, error) {
	var result T
	err := r.Query(ctx).First(&result, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindBy retrieves the first record matching a condition.
func (r *Repository[T]) FindBy(ctx context.Context, col string, val any) (*T, error) {
	var result T
	err := r.Query(ctx).Where(col+" = ?", val).First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// All retrieves all records for the model type, executing the query chain if provided.
// It accepts optional Gorm db instances to continue a chain.
func (r *Repository[T]) All(ctx context.Context, query ...*gorm.DB) ([]T, error) {
	var results []T
	q := r.Query(ctx)
	if len(query) > 0 {
		q = query[0]
	}
	err := q.Find(&results).Error
	return results, err
}

// Create inserts a new record and returns it.
func (r *Repository[T]) Create(ctx context.Context, data *T) (*T, error) {
	err := r.Query(ctx).Create(data).Error
	return data, err
}

// Update modifies an existing record by ID and returns the updated record.
func (r *Repository[T]) Update(ctx context.Context, id any, updates map[string]any) (*T, error) {
	var model T
	err := r.Query(ctx).Where("id = ?", id).First(&model).Updates(updates).Error
	if err != nil {
		return nil, err
	}
	return r.Find(ctx, id)
}

// Delete removes a record by ID.
func (r *Repository[T]) Delete(ctx context.Context, id any) error {
	var model T
	return r.Query(ctx).Where("id = ?", id).Delete(&model).Error
}

// Paginate executes a paginated query returning a structured response.
func (r *Repository[T]) Paginate(ctx context.Context, page, perPage int, query ...*gorm.DB) (Paginated[T], error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	q := r.Query(ctx)
	if len(query) > 0 {
		q = query[0]
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return Paginated[T]{}, err
	}

	offset := (page - 1) * perPage
	var results []T
	if err := q.Limit(perPage).Offset(offset).Find(&results).Error; err != nil {
		return Paginated[T]{}, err
	}

	lastPage := int(total) / perPage
	if int(total)%perPage != 0 {
		lastPage++
	}

	return Paginated[T]{
		Data:     results,
		Total:    int(total),
		Page:     page,
		PerPage:  perPage,
		LastPage: lastPage,
	}, nil
}

