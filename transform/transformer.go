// Package transform provides API resource transformation helpers.
// Use transformers to control exactly which fields are exposed in JSON responses,
// preventing accidental leaking of sensitive data (passwords, internal IDs, etc.).
package transform

import (
	"github.com/astraframework/astra/db"
)

// Transformer defines the interface for converting a model to a response map.
type Transformer[T any] interface {
	Transform(item T) map[string]any
}

// Item transforms a single model using the given Transformer.
//
// Example:
//
//	return ctx.JSON(transform.Item(user, &UserTransformer{}))
func Item[T any](item T, t Transformer[T]) map[string]any {
	return t.Transform(item)
}

// ItemP transforms a pointer to a model, returning nil if the pointer is nil.
func ItemP[T any](item *T, t Transformer[T]) map[string]any {
	if item == nil {
		return nil
	}
	return t.Transform(*item)
}

// Collection transforms a slice of models using the given Transformer.
//
// Example:
//
//	return ctx.JSON(transform.Collection(users, &UserTransformer{}))
func Collection[T any](items []T, t Transformer[T]) []map[string]any {
	result := make([]map[string]any, len(items))
	for i, item := range items {
		result[i] = t.Transform(item)
	}
	return result
}

// Paginated transforms a db.Paginated[T] result, embedding transformed data
// alongside the pagination metadata.
//
// Example:
//
//	paginated, _ := qb.Paginate(ctx, page, perPage)
//	return ctx.JSON(transform.Paginated(paginated, &UserTransformer{}))
func Paginated[T any](p db.Paginated[T], t Transformer[T]) map[string]any {
	return map[string]any{
		"data":      Collection(p.Data, t),
		"total":     p.Total,
		"page":      p.Page,
		"per_page":  p.PerPage,
		"last_page": p.LastPage,
	}
}

// CursorPaginated transforms a db.CursorPaginated[T] result.
func CursorPaginated[T any](p db.CursorPaginated[T], t Transformer[T]) map[string]any {
	return map[string]any{
		"data":        Collection(p.Data, t),
		"next_cursor": p.NextCursor,
		"has_more":    p.HasMore,
	}
}

// FuncTransformer wraps a plain function as a Transformer[T].
// Useful for one-off transformations without declaring a type.
//
// Example:
//
//	t := transform.Func(func(u User) map[string]any { return map[string]any{"id": u.ID} })
//	return ctx.JSON(transform.Item(user, t))
type FuncTransformer[T any] struct {
	fn func(T) map[string]any
}

// Func creates a Transformer[T] from a plain function.
func Func[T any](fn func(T) map[string]any) Transformer[T] {
	return &FuncTransformer[T]{fn: fn}
}

func (f *FuncTransformer[T]) Transform(item T) map[string]any {
	return f.fn(item)
}
