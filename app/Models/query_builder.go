package models

import (
	"math"

	"gorm.io/gorm"
)

// QueryBuilder provides a chainable query interface wrapping GORM.
// Mirrors Astra's Lucid query builder:
//
//	const users = await User.query()
//	  .where('age', '>', 18)
//	  .orderBy('name', 'asc')
//	  .preload('posts')
//	  .paginate(1, 10)
//
// Go equivalent:
//
//	users, err := models.Query[User](db).
//	  Where("age > ?", 18).
//	  OrderBy("name", "asc").
//	  Preload("Posts").
//	  Paginate(1, 10)
type QueryBuilder[T any] struct {
	db *gorm.DB
}

// Query starts a new query builder for the given model type.
// Mirrors: User.query()
func Query[T any](db *gorm.DB) *QueryBuilder[T] {
	var model T
	return &QueryBuilder[T]{
		db: db.Model(&model),
	}
}

// Where adds a WHERE clause.
// Mirrors: .where('column', 'operator', value)
func (q *QueryBuilder[T]) Where(query string, args ...any) *QueryBuilder[T] {
	q.db = q.db.Where(query, args...)
	return q
}

// WhereIn adds a WHERE IN clause.
func (q *QueryBuilder[T]) WhereIn(column string, values []any) *QueryBuilder[T] {
	q.db = q.db.Where(column+" IN ?", values)
	return q
}

// WhereNull adds a WHERE IS NULL clause.
func (q *QueryBuilder[T]) WhereNull(column string) *QueryBuilder[T] {
	q.db = q.db.Where(column + " IS NULL")
	return q
}

// WhereNotNull adds a WHERE IS NOT NULL clause.
func (q *QueryBuilder[T]) WhereNotNull(column string) *QueryBuilder[T] {
	q.db = q.db.Where(column + " IS NOT NULL")
	return q
}

// OrWhere adds an OR WHERE clause.
func (q *QueryBuilder[T]) OrWhere(query string, args ...any) *QueryBuilder[T] {
	q.db = q.db.Or(query, args...)
	return q
}

// OrderBy adds an ORDER BY clause.
// Mirrors: .orderBy('column', 'asc'|'desc')
func (q *QueryBuilder[T]) OrderBy(column string, direction string) *QueryBuilder[T] {
	q.db = q.db.Order(column + " " + direction)
	return q
}

// Limit sets the LIMIT.
func (q *QueryBuilder[T]) Limit(limit int) *QueryBuilder[T] {
	q.db = q.db.Limit(limit)
	return q
}

// Offset sets the OFFSET.
func (q *QueryBuilder[T]) Offset(offset int) *QueryBuilder[T] {
	q.db = q.db.Offset(offset)
	return q
}

// Preload eager-loads a relationship.
// Mirrors: .preload('posts')
func (q *QueryBuilder[T]) Preload(relation string, args ...any) *QueryBuilder[T] {
	q.db = q.db.Preload(relation, args...)
	return q
}

// Select specifies columns to select.
func (q *QueryBuilder[T]) Select(columns ...string) *QueryBuilder[T] {
	q.db = q.db.Select(columns)
	return q
}

// Joins adds a JOIN clause.
func (q *QueryBuilder[T]) Joins(join string, args ...any) *QueryBuilder[T] {
	q.db = q.db.Joins(join, args...)
	return q
}

// Group adds a GROUP BY clause.
func (q *QueryBuilder[T]) Group(column string) *QueryBuilder[T] {
	q.db = q.db.Group(column)
	return q
}

// Having adds a HAVING clause.
func (q *QueryBuilder[T]) Having(query string, args ...any) *QueryBuilder[T] {
	q.db = q.db.Having(query, args...)
	return q
}

// Distinct adds DISTINCT.
func (q *QueryBuilder[T]) Distinct(columns ...string) *QueryBuilder[T] {
	args := make([]any, len(columns))
	for i, col := range columns {
		args[i] = col
	}
	q.db = q.db.Distinct(args...)
	return q
}

// Scopes applies one or more query scopes.
// Mirrors Astra's query scopes feature.
func (q *QueryBuilder[T]) Scopes(scopes ...func(*gorm.DB) *gorm.DB) *QueryBuilder[T] {
	q.db = q.db.Scopes(scopes...)
	return q
}

// ══════════════════════════════════════════════════════════════════════
// Terminal Methods (execute the query and return results)
// ══════════════════════════════════════════════════════════════════════

// First returns the first matching record.
// Mirrors: .first()
func (q *QueryBuilder[T]) First() (*T, error) {
	var model T
	result := q.db.First(&model)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

// All returns all matching records.
// Mirrors: .fetch() in Lucid
func (q *QueryBuilder[T]) All() ([]T, error) {
	var models []T
	result := q.db.Find(&models)
	if result.Error != nil {
		return nil, result.Error
	}
	return models, nil
}

// Count returns the count of matching records.
func (q *QueryBuilder[T]) Count() (int64, error) {
	var count int64
	result := q.db.Count(&count)
	return count, result.Error
}

// Exists returns true if any matching record exists.
func (q *QueryBuilder[T]) Exists() (bool, error) {
	count, err := q.Count()
	return count > 0, err
}

// Paginate returns paginated results.
// Mirrors: .paginate(page, perPage)
func (q *QueryBuilder[T]) Paginate(page int, perPage int) (*PaginatedResult[T], error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	var total int64
	q.db.Count(&total)

	var models []T
	offset := (page - 1) * perPage
	result := q.db.Offset(offset).Limit(perPage).Find(&models)
	if result.Error != nil {
		return nil, result.Error
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	return &PaginatedResult[T]{
		Data:        models,
		Total:       total,
		PerPage:     perPage,
		CurrentPage: page,
		LastPage:    lastPage,
		HasMore:     page < lastPage,
	}, nil
}

// Update updates matching records with the given values.
// Mirrors: .update({ name: 'John' })
func (q *QueryBuilder[T]) Update(values map[string]any) error {
	var model T
	return q.db.Model(&model).Updates(values).Error
}

// DeleteAll deletes all matching records.
// Mirrors: .delete()
func (q *QueryBuilder[T]) DeleteAll() (int64, error) {
	var model T
	result := q.db.Delete(&model)
	return result.RowsAffected, result.Error
}

// Pluck retrieves a single column's values.
func (q *QueryBuilder[T]) Pluck(column string) ([]any, error) {
	var values []any
	result := q.db.Pluck(column, &values)
	return values, result.Error
}

// Raw returns the underlying GORM DB for advanced usage.
func (q *QueryBuilder[T]) Raw() *gorm.DB {
	return q.db
}

// ══════════════════════════════════════════════════════════════════════
// Pagination Result
// ══════════════════════════════════════════════════════════════════════

// PaginatedResult holds paginated query results.
// Mirrors Lucid's SimplePaginator.
type PaginatedResult[T any] struct {
	Data        []T   `json:"data"`
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	HasMore     bool  `json:"has_more"`
}

// ══════════════════════════════════════════════════════════════════════
// Common Query Scopes (reusable)
// ══════════════════════════════════════════════════════════════════════

// ScopeActive is a scope that filters for active records.
func ScopeActive(db *gorm.DB) *gorm.DB {
	return db.Where("active = ?", true)
}

// ScopeRecent is a scope that orders by created_at desc.
func ScopeRecent(db *gorm.DB) *gorm.DB {
	return db.Order("created_at DESC")
}

// ScopeByUser is a scope factory for filtering by user_id.
func ScopeByUser(userID uint) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("user_id = ?", userID)
	}
}
