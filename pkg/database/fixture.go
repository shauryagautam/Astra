package database

import (
	"context"
	"reflect"

	"github.com/brianvoe/gofakeit/v6"
)

// FixtureDefinition describes deterministic defaults and named overrides for
// test data. It is intentionally explicit and never stores package-global
// state.
type FixtureDefinition[T any] struct {
	attrs  map[string]any
	states map[string]map[string]any
}

// FixtureBuilder constructs test models and may persist them when given a DB.
type FixtureBuilder[T any] struct {
	db    *DB
	def   *FixtureDefinition[T]
	state string
}

// Fixture creates a builder for T.
func Fixture[T any](fn func(*FixtureDefinition[T])) *FixtureBuilder[T] {
	def := &FixtureDefinition[T]{
		attrs:  make(map[string]any),
		states: make(map[string]map[string]any),
	}
	if fn != nil {
		fn(def)
	}
	return &FixtureBuilder[T]{def: def}
}

func (f *FixtureDefinition[T]) Set(field string, value any) {
	f.attrs[field] = value
}

func (f *FixtureDefinition[T]) State(name string, attrs map[string]any) {
	f.states[name] = attrs
}

func (fb *FixtureBuilder[T]) State(name string) *FixtureBuilder[T] {
	fb.state = name
	return fb
}

func (fb *FixtureBuilder[T]) WithDB(db *DB) *FixtureBuilder[T] {
	fb.db = db
	return fb
}

func (fb *FixtureBuilder[T]) Make() T {
	var model T
	val := reflect.ValueOf(&model).Elem()
	attrs := make(map[string]any, len(fb.def.attrs))
	for k, v := range fb.def.attrs {
		attrs[k] = v
	}
	if stateAttrs, ok := fb.def.states[fb.state]; ok {
		for k, v := range stateAttrs {
			attrs[k] = v
		}
	}
	for field, value := range attrs {
		f := val.FieldByName(field)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		if fn, ok := value.(func() any); ok {
			value = fn()
		}
		fv := reflect.ValueOf(value)
		if fv.IsValid() && fv.Type().AssignableTo(f.Type()) {
			f.Set(fv)
		}
	}
	return model
}

func (fb *FixtureBuilder[T]) Create(ctx context.Context, db ...*DB) (*T, error) {
	target := fb.db
	if len(db) > 0 {
		target = db[0]
	}
	if target == nil {
		return nil, ErrMissingDatabase
	}
	model := fb.Make()
	return Query[T](target).Create(&model, ctx)
}

func (fb *FixtureBuilder[T]) CreateMany(ctx context.Context, count int, db ...*DB) ([]*T, error) {
	if count < 0 {
		return nil, ErrInvalidFixtureCount
	}
	target := fb.db
	if len(db) > 0 {
		target = db[0]
	}
	if target == nil {
		return nil, ErrMissingDatabase
	}
	models := make([]*T, count)
	for i := range models {
		model, err := fb.Create(ctx, target)
		if err != nil {
			return nil, err
		}
		models[i] = model
	}
	return models, nil
}

// Fake helpers are deterministic test-data conveniences; they do not affect
// ORM or application state.
func FakeName() string           { return gofakeit.Name() }
func FakeEmail() string          { return gofakeit.Email() }
func FakePhone() string          { return gofakeit.Phone() }
func FakeAddress() string        { return gofakeit.Address().Address }
func FakeCompany() string        { return gofakeit.Company() }
func FakeText(length int) string { return gofakeit.LetterN(uint(length)) }
func FakeInt(min, max int) int   { return gofakeit.IntRange(min, max) }
func FakeBool() bool             { return gofakeit.Bool() }
