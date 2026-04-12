package auth

import (
	"time"

	"github.com/shauryagautam/Astra/pkg/engine/config"
)

// Standalone constructors let you use the Astra auth package in any Go project
// without pulling in engine.App. Add to your go.mod with:
//
//	go get github.com/shauryagautam/Astra/pkg/auth
//
// JWT usage (stateless, no Redis required):
//
//	mgr, err := auth.NewJWTStandalone("super-secret-key-min-32-chars!!!!")
//	pair, err := mgr.IssueTokenPair(ctx, userID, nil)
//	claims, err := mgr.Verify(pair.AccessToken)
//
// Cookie session guard usage (requires a SessionDriver):
//
//	guard := auth.NewCookieGuard(mySessionDriver)
//	if err := guard.Attempt(requestCtx); err != nil { /* unauthorized */ }

// JWTOption is a functional option for configuring a standalone JWTManager.
type JWTOption func(*config.AuthConfig)

// WithIssuer sets the "iss" claim in issued tokens.
func WithIssuer(issuer string) JWTOption {
	return func(cfg *config.AuthConfig) { cfg.JWTIssuer = issuer }
}

// WithAccessTokenExpiry sets the access token TTL.
func WithAccessTokenExpiry(d time.Duration) JWTOption {
	return func(cfg *config.AuthConfig) { cfg.AccessTokenExpiry = d }
}

// WithRefreshTokenExpiry sets the refresh token TTL.
func WithRefreshTokenExpiry(d time.Duration) JWTOption {
	return func(cfg *config.AuthConfig) { cfg.RefreshTokenExpiry = d }
}

// NewJWTStandalone creates a JWTManager using only a secret string.
// No Redis client is required for pure access-token verification.
// Refresh-token rotation is disabled when Redis is nil — issue single-use tokens only.
//
// The secret must be at least 32 characters for HS256 security.
//
//	mgr, err := auth.NewJWTStandalone("super-secret-key-min-32-chars!!!!", auth.WithIssuer("myapp"))
func NewJWTStandalone(secret string, opts ...JWTOption) (*JWTManager, error) {
	if len(secret) < 32 {
		return nil, &StandaloneError{"NewJWTStandalone: secret must be at least 32 characters (got " + string(rune(len(secret)+48)) + ")"}
	}

	cfg := config.AuthConfig{
		JWTSecret:          secret,
		JWTIssuer:          "astra",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// nil Redis — refresh token rotation is disabled; IssueTokenPair skips Redis writes.
	return NewJWTManager(cfg, nil), nil
}

// MustJWTStandalone is like NewJWTStandalone but panics on error.
// Use at program startup where a misconfigured secret is a fatal condition.
func MustJWTStandalone(secret string, opts ...JWTOption) *JWTManager {
	mgr, err := NewJWTStandalone(secret, opts...)
	if err != nil {
		panic(err)
	}
	return mgr
}

// StandaloneError is returned by standalone constructors for configuration errors.
type StandaloneError struct{ msg string }

func (e *StandaloneError) Error() string { return "auth: " + e.msg }
