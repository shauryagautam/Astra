package db

import "context"

// BeforeCreateHook is implemented by models that need to run logic before creation.
type BeforeCreateHook interface {
	BeforeCreate(ctx context.Context) error
}

// AfterCreateHook is implemented by models that need to run logic after creation.
type AfterCreateHook interface {
	AfterCreate(ctx context.Context) error
}

// BeforeUpdateHook is implemented by models that need to run logic before updating.
type BeforeUpdateHook interface {
	BeforeUpdate(ctx context.Context) error
}

// AfterUpdateHook is implemented by models that need to run logic after updating.
type AfterUpdateHook interface {
	AfterUpdate(ctx context.Context) error
}

// BeforeDeleteHook is implemented by models that need to run logic before deletion.
type BeforeDeleteHook interface {
	BeforeDelete(ctx context.Context) error
}

// AfterDeleteHook is implemented by models that need to run logic after deletion.
type AfterDeleteHook interface {
	AfterDelete(ctx context.Context) error
}
