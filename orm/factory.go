package orm

import (
	"context"
	"reflect"

	"github.com/brianvoe/gofakeit/v6"
)

// FactoryDef defines how to build a model
type FactoryDef[T any] struct {
	attrs  map[string]any
	states map[string]map[string]any
}

// FactoryBuilder builds factory instances
type FactoryBuilder[T any] struct {
	db    *DB
	def   *FactoryDef[T]
	state string
}

// Factory creates a new factory for type T
func Factory[T any](fn func(*FactoryDef[T])) *FactoryBuilder[T] {
	def := &FactoryDef[T]{
		attrs:  make(map[string]any),
		states: make(map[string]map[string]any),
	}
	fn(def)
	return &FactoryBuilder[T]{def: def}
}

// Set sets a field value in the factory definition
func (f *FactoryDef[T]) Set(field string, value any) {
	f.attrs[field] = value
}

// State defines a named state with overrides
func (f *FactoryDef[T]) State(name string, attrs map[string]any) {
	f.states[name] = attrs
}

// State applies a named state to a builder
func (fb *FactoryBuilder[T]) State(name string) *FactoryBuilder[T] {
	fb.state = name
	return fb
}

// WithDB associates the factory with a database for persisting
func (fb *FactoryBuilder[T]) WithDB(db *DB) *FactoryBuilder[T] {
	fb.db = db
	return fb
}

// Make creates an instance without persisting
func (fb *FactoryBuilder[T]) Make() T {
	var model T
	val := reflect.ValueOf(&model).Elem()

	attrs := make(map[string]any)
	for k, v := range fb.def.attrs {
		attrs[k] = v
	}

	if fb.state != "" {
		if stateAttrs, ok := fb.def.states[fb.state]; ok {
			for k, v := range stateAttrs {
				attrs[k] = v
			}
		}
	}

	for field, value := range attrs {
		f := val.FieldByName(field)
		if !f.IsValid() || !f.CanSet() {
			continue
		}

		var v any
		if fn, ok := value.(func() any); ok {
			v = fn()
		} else {
			v = value
		}

		fv := reflect.ValueOf(v)
		if fv.Type().AssignableTo(f.Type()) {
			f.Set(fv)
		}
	}

	return model
}

// Create creates and persists an instance
func (fb *FactoryBuilder[T]) Create(ctx context.Context, db ...*DB) (*T, error) {
	database := fb.db
	if len(db) > 0 {
		database = db[0]
	}
	if database == nil {
		panic("orm: FactoryBuilder.Create requires a DB. Use WithDB() or pass it to Create()")
	}

	model := fb.Make()
	return Query[T](database).Create(model, ctx)
}

// CreateMany creates and persists multiple instances
func (fb *FactoryBuilder[T]) CreateMany(ctx context.Context, count int, db ...*DB) ([]*T, error) {
	database := fb.db
	if len(db) > 0 {
		database = db[0]
	}

	models := make([]*T, count)
	for i := 0; i < count; i++ {
		model, err := fb.Create(ctx, database)
		if err != nil {
			return nil, err
		}
		models[i] = model
	}
	return models, nil
}

// Fake helpers (exposed globally for convenience)

func FakeName() string           { return gofakeit.Name() }
func FakeEmail() string          { return gofakeit.Email() }
func FakePhone() string          { return gofakeit.Phone() }
func FakeAddress() string        { return gofakeit.Address().Address }
func FakeCompany() string        { return gofakeit.Company() }
func FakeText(length int) string { return gofakeit.LetterN(uint(length)) }
func FakeInt(min, max int) int   { return gofakeit.IntRange(min, max) }
func FakeBool() bool             { return gofakeit.Bool() }
