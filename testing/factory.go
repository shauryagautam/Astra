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

	"github.com/astraframework/astra/orm"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/require"
)

// Factory defines an interface for creating models in tests or seeders.
type Factory[T any] interface {
	Create(ctx context.Context, db *orm.DB) (*T, error)
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

type ModelFactory[T any] struct {
	fields    map[string]func() any // field name (lowercase) -> value generator
	count     int
	tableName string
	useFakes  bool
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

// WithFakes enables automatic fake data generation for unassigned fields based on struct tags.
func (f *ModelFactory[T]) WithFakes() *ModelFactory[T] {
	f.useFakes = true
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
func (f *ModelFactory[T]) Create(ctx context.Context, db *orm.DB) (*T, error) {
	model := f.Make()
	return orm.Query[T](db).Table(f.tableName).Create(*model, ctx)
}

// CreateMany inserts count instances into the database and returns all created records.
func (f *ModelFactory[T]) CreateMany(ctx context.Context, db *orm.DB) ([]*T, error) {
	results := make([]*T, 0, f.count)
	for i := 0; i < f.count; i++ {
		model := f.Make()
		rec, err := orm.Query[T](db).Table(f.tableName).Create(*model, ctx)
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

	// If fakes enabled, prepopulate with gofakeit
	if f.useFakes {
		var zero T
		typ := reflect.TypeOf(zero)
		if typ.Kind() == reflect.Struct {
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				name := fieldTagName(field)

				// Try to get value from gofakeit tag
				if fakeTag := field.Tag.Get("fake"); fakeTag != "" {
					if fakeTag != "skip" {
						out[name] = gofakeit.Generate(fakeTag)
					}
				} else {
					// Fallback to name-based or type-based generation if no tag?
					// For now, only use explicit tags to avoid over-generation.
				}
			}
		}
	}

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
