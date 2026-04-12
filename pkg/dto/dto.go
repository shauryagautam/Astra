// Package dto provides Data Transfer Object (DTO) mapping helpers.
// Use mappers to control exactly which fields are exposed in JSON responses,
// preventing accidental leaking of sensitive data (passwords, internal IDs, etc.).
package dto

import (
	"github.com/shauryagautam/Astra/pkg/database"
)

// Mapper defines the interface for converting a model to a Data Transfer Object (map).
type Mapper[T any] interface {
	ToDTO(item T) map[string]any
}

// MapItem transforms a single model to its DTO representation using the given Mapper.
//
// Example:
//
//	return ctx.JSON(dto.MapItem(user, &UserMapper{}))
func MapItem[T any](item T, m Mapper[T]) map[string]any {
	return m.ToDTO(item)
}

// MapItemP transforms a pointer to a model to its DTO, returning nil if the pointer is nil.
func MapItemP[T any](item *T, m Mapper[T]) map[string]any {
	if item == nil {
		return nil
	}
	return m.ToDTO(*item)
}

// MapCollection transforms a slice of models to a slice of DTOs using the given Mapper.
//
// Example:
//
//	return ctx.JSON(dto.MapCollection(users, &UserMapper{}))
func MapCollection[T any](items []T, m Mapper[T]) []map[string]any {
	result := make([]map[string]any, len(items))
	for i, item := range items {
		result[i] = m.ToDTO(item)
	}
	return result
}

// MapPaginated transforms a database.PaginationResult[T] to a paginated DTO response.
//
// Example:
//
//	paginated, _ := qb.Paginate(ctx, page, perPage)
//	return ctx.JSON(dto.MapPaginated(paginated, &UserMapper{}))
func MapPaginated[T any](p database.PaginationResult[T], m Mapper[T]) map[string]any {
	return map[string]any{
		"data":      MapCollection(p.Data, m),
		"total":     p.Total,
		"page":      p.CurrentPage,
		"per_page":  p.PerPage,
		"last_page": p.LastPage,
	}
}

// MapCursorPaginated transforms a database.CursorPaginated[T] result to a cursor-paginated DTO response.
func MapCursorPaginated[T any](p database.CursorPaginated[T], m Mapper[T]) map[string]any {
	return map[string]any{
		"data":        MapCollection(p.Data, m),
		"next_cursor": p.NextCursor,
		"has_more":    p.HasMore,
	}
}

// FuncMapper wraps a plain function as a Mapper[T].
// Useful for one-off mappings without declaring a type.
//
// Example:
//
//	m := dto.MapFunc(func(u User) map[string]any { return map[string]any{"id": u.ID} })
//	return ctx.JSON(dto.MapItem(user, m))
type FuncMapper[T any] struct {
	fn func(T) map[string]any
}

// MapFunc creates a Mapper[T] from a plain function.
func MapFunc[T any](fn func(T) map[string]any) Mapper[T] {
	return &FuncMapper[T]{fn: fn}
}

func (f *FuncMapper[T]) ToDTO(item T) map[string]any {
	return f.fn(item)
}
