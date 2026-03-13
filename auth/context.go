package auth

import (
	"context"
)

// ContextKey is a custom type to prevent context key collisions.
type ContextKey string

// UserKey is the typed key under which auth claims are stored in standard contexts.
const UserKey ContextKey = "astra_auth_user"

// GetAuthUser retrieves the authenticated user from a standard context.Context.
func GetAuthUser(ctx context.Context) *AuthClaims {
	claims, _ := ctx.Value(UserKey).(*AuthClaims)
	return claims
}
