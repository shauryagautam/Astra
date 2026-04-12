package database

import "context"

// Lifecycle hook interfaces
type BeforeCreateHook interface {
	BeforeCreate(ctx context.Context, db *DB) error
}

type AfterCreateHook interface {
	AfterCreate(ctx context.Context, db *DB) error
}

type BeforeUpdateHook interface {
	BeforeUpdate(ctx context.Context, db *DB) error
}

type AfterUpdateHook interface {
	AfterUpdate(ctx context.Context, db *DB) error
}

type BeforeDeleteHook interface {
	BeforeDelete(ctx context.Context, db *DB) error
}

type AfterDeleteHook interface {
	AfterDelete(ctx context.Context, db *DB) error
}

type AfterFindHook interface {
	AfterFind(ctx context.Context, db *DB) error
}

func callBeforeCreate[T any](ctx context.Context, db *DB, model *T) error {
	if h, ok := any(model).(BeforeCreateHook); ok {
		return h.BeforeCreate(ctx, db)
	}
	return nil
}

func callAfterCreate[T any](ctx context.Context, db *DB, model *T) error {
	// Special handling for Auditable trait
	if a, ok := any(model).(Auditable); ok {
		_ = a.AfterCreate(ctx, db, model)
	}

	if h, ok := any(model).(AfterCreateHook); ok {
		return h.AfterCreate(ctx, db)
	}
	return nil
}

func callBeforeUpdate[T any](ctx context.Context, db *DB, model *T) error {
	if h, ok := any(model).(BeforeUpdateHook); ok {
		return h.BeforeUpdate(ctx, db)
	}
	return nil
}

func callAfterUpdate[T any](ctx context.Context, db *DB, model *T) error {
	// Special handling for Auditable trait
	if a, ok := any(model).(Auditable); ok {
		_ = a.AfterUpdate(ctx, db, model)
	}

	if h, ok := any(model).(AfterUpdateHook); ok {
		return h.AfterUpdate(ctx, db)
	}
	return nil
}

func callAfterFind[T any](ctx context.Context, db *DB, model *T) error {
	if h, ok := any(model).(AfterFindHook); ok {
		return h.AfterFind(ctx, db)
	}
	return nil
}
