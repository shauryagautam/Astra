package contracts

import "context"

// Authenticatable is the interface that user models must implement
// to work with the auth system. Mirrors AdonisJS's User model requirements.
type Authenticatable interface {
	// GetAuthIdentifier returns the unique identifier (usually primary key).
	GetAuthIdentifier() any

	// GetAuthIdentifierName returns the column name of the identifier.
	GetAuthIdentifierName() string

	// GetRememberMeToken returns the remember-me token if any.
	GetRememberMeToken() string

	// SetRememberMeToken sets the remember-me token.
	SetRememberMeToken(token string)
}

// UserProviderContract finds users for authentication purposes.
// Mirrors AdonisJS's UserProviderContract.
type UserProviderContract interface {
	// FindById finds a user by their unique identifier.
	FindById(ctx context.Context, id any) (Authenticatable, error)

	// FindByToken finds a user by a specific token type and value.
	FindByToken(ctx context.Context, tokenType string, token string) (Authenticatable, error)

	// FindByCredentials finds a user by login credentials (e.g., email).
	FindByCredentials(ctx context.Context, credentials map[string]any) (Authenticatable, error)
}

// TokenContract represents an authentication token.
type TokenContract interface {
	// Type returns the token type (e.g., "bearer", "opaque").
	Type() string

	// Name returns an optional token name.
	Name() string

	// Token returns the raw token string.
	Token() string

	// ExpiresAt returns the expiration timestamp (Unix).
	// Returns 0 if the token never expires.
	ExpiresAt() int64
}

// GuardContract defines the interface for an authentication guard.
// Guards handle the actual authentication logic (JWT, OAT, Session, etc.).
// Mirrors AdonisJS's GuardContract.
type GuardContract interface {
	// Attempt authenticates via credentials and returns a token.
	// Mirrors: await auth.attempt(email, password)
	Attempt(ctx HttpContextContract, credentials map[string]any) (TokenContract, error)

	// Login authenticates a known user and returns a token.
	// Mirrors: await auth.login(user)
	Login(ctx HttpContextContract, user Authenticatable) (TokenContract, error)

	// Authenticate verifies the request and loads the user.
	// Mirrors: await auth.authenticate()
	Authenticate(ctx HttpContextContract) (Authenticatable, error)

	// Check returns true if the request is authenticated (does not throw).
	// Mirrors: await auth.check()
	Check(ctx HttpContextContract) bool

	// Logout invalidates the current auth token.
	// Mirrors: await auth.logout()
	Logout(ctx HttpContextContract) error

	// User returns the authenticated user (nil if not authenticated).
	User() Authenticatable
}

// AuthManagerContract manages multiple auth guards.
// Mirrors AdonisJS's Auth module.
type AuthManagerContract interface {
	// Use selects an auth guard by name.
	// Mirrors: auth.use('jwt'), auth.use('api')
	Use(guard string) GuardContract

	// DefaultGuard returns the default guard.
	DefaultGuard() GuardContract
}
