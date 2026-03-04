// Package testing provides model factories for use in tests and seed data.
// Factories use a fluent builder API: Set field values, then Make (in-memory)
// or Create (insert into DB).
package testing

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Factory defines an interface for creating models in tests.
type Factory[T any] interface {
	Create(ctx context.Context, tx pgx.Tx) *T
	Make() *T
}

// BaseFactory provides helper methods for tests.
type BaseFactory struct {
	t *testing.T
}

// NewBaseFactory creates a new BaseFactory.
func NewBaseFactory(t *testing.T) *BaseFactory {
	return &BaseFactory{t: t}
}

// RequireNoError asserts that err is nil.
func (f *BaseFactory) RequireNoError(err error) {
	require.NoError(f.t, err)
}

// ─── Fluent Factory Builder ───────────────────────────────────────────────────

// ModelFactory is a fluent builder for constructing model instances.
// T must be a struct type.
type ModelFactory[T any] struct {
	fields    map[string]func() any // field name (lowercase) -> value generator
	count     int
	tableName string
}

// NewFactory creates a new ModelFactory[T] for the given database table.
//
// Example:
//
//	factory := testing.NewFactory[User]("users").
//	    Set("name", faker.Name()).
//	    Set("email", faker.Email()).
//	    Count(10)
//
//	users := factory.CreateMany(ctx, db)
func NewFactory[T any](table string) *ModelFactory[T] {
	return &ModelFactory[T]{
		fields:    make(map[string]func() any),
		count:     1,
		tableName: table,
	}
}

// Set registers a fixed value for the given field name.
// The field name should match the struct's db/json tag name.
func (f *ModelFactory[T]) Set(field string, value any) *ModelFactory[T] {
	f.fields[field] = func() any { return value }
	return f
}

// SetFunc registers a generator function for the given field.
// The function is called once per created instance, enabling unique values.
func (f *ModelFactory[T]) SetFunc(field string, fn func() any) *ModelFactory[T] {
	f.fields[field] = fn
	return f
}

// Count sets the number of instances to create/make.
func (f *ModelFactory[T]) Count(n int) *ModelFactory[T] {
	f.count = n
	return f
}

// Make returns a single in-memory instance (no DB call).
func (f *ModelFactory[T]) Make() *T {
	return applyFields[T](f.currentValues())
}

// MakeMany returns count in-memory instances.
func (f *ModelFactory[T]) MakeMany() []*T {
	results := make([]*T, f.count)
	for i := range results {
		results[i] = applyFields[T](f.currentValues())
	}
	return results
}

// Create inserts a single instance into the database and returns the created record.
func (f *ModelFactory[T]) Create(ctx context.Context, db *pgxpool.Pool) (*T, error) {
	data := f.currentValues()
	return insertOne[T](ctx, db, f.tableName, data)
}

// CreateMany inserts count instances into the database and returns all created records.
func (f *ModelFactory[T]) CreateMany(ctx context.Context, db *pgxpool.Pool) ([]*T, error) {
	results := make([]*T, 0, f.count)
	for i := 0; i < f.count; i++ {
		data := f.currentValues()
		rec, err := insertOne[T](ctx, db, f.tableName, data)
		if err != nil {
			return results, fmt.Errorf("factory.CreateMany: failed on record %d: %w", i, err)
		}
		results = append(results, rec)
	}
	return results, nil
}

// currentValues evaluates all registered generators and returns a snapshot.
func (f *ModelFactory[T]) currentValues() map[string]any {
	out := make(map[string]any, len(f.fields))
	for k, fn := range f.fields {
		out[k] = fn()
	}
	return out
}

// applyFields creates a T and sets fields by name using struct tag reflection.
func applyFields[T any](data map[string]any) *T {
	var zero T
	v := reflect.New(reflect.TypeOf(zero)).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// Try db tag first, then json tag, then lowercase field name
		name := fieldTagName(field)
		if val, ok := data[name]; ok {
			fv := v.Field(i)
			rv := reflect.ValueOf(val)
			if rv.Type().AssignableTo(fv.Type()) {
				fv.Set(rv)
			} else if rv.Type().ConvertibleTo(fv.Type()) {
				fv.Set(rv.Convert(fv.Type()))
			}
		}
	}

	result := v.Interface().(T)
	return &result
}

func fieldTagName(f reflect.StructField) string {
	if tag := f.Tag.Get("db"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	if tag := f.Tag.Get("json"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return strings.ToLower(f.Name)
}

// insertOne inserts a single record and returns it using RETURNING *.
func insertOne[T any](ctx context.Context, db *pgxpool.Pool, table string, data map[string]any) (*T, error) {
	cols := make([]string, 0, len(data))
	vals := make([]any, 0, len(data))
	placeholders := make([]string, 0, len(data))
	i := 1
	for k, v := range data {
		cols = append(cols, k)
		vals = append(vals, v)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		i++
	}
	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		table,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)
	rows, err := db.Query(ctx, query, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, err
	}
	return &res, nil
}
