package graphql

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/auth"
)

// Directive is a function that can wrap a resolver execution.
type Directive func(ctx context.Context, next func(ctx context.Context) (interface{}, error)) (interface{}, error)

// AuthDirective implements the @auth directive.
// It checks if a user is authenticated in the context.
func AuthDirective(ctx context.Context, next func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	user := auth.GetAuthUser(ctx)
	if user == nil {
		return nil, fmt.Errorf("unauthenticated")
	}
	return next(ctx)
}

// NOTE: graphql-go doesn't support directives out of the box easily
// without custom wrapping. We provide these as helpers for now or
// will implement a custom Resolve function if needed.
