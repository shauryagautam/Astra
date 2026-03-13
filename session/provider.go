package session

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/astraframework/astra/core"
)

// Config holds session package configuration.
type Config struct {
	// Store is the session backend. Required.
	Store Store

	// CookieName overrides the default session cookie name ("astra_session").
	CookieName string

	// MaxAge is the session lifetime (default 24h).
	MaxAge time.Duration

	// Secure sets the Secure flag on the session cookie.
	// Defaults to true in production (APP_ENV != "development").
	Secure bool

	// SameSite controls the SameSite cookie attribute (default: Lax).
	SameSite http.SameSite
}

// Provider implements core.Provider for the session package.
// It registers the session middleware and the session store as "session".
//
// Example:
//
//	app.Use(session.NewProvider(session.Config{
//	    Store: session.NewCookieStore([]byte(os.Getenv("APP_KEY"))),
//	}))
type Provider struct {
	core.BaseProvider
	cfg Config
}

// NewProvider creates a session Provider with the given config.
func NewProvider(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

// Register stores the session store as "session" in the service container.
func (p *Provider) Register(a *core.App) error {
	if p.cfg.Store == nil {
		appKey := a.Env.String("APP_KEY", "")
		if appKey == "" {
			return fmt.Errorf("session: APP_KEY is not set and no Store was provided")
		}

		secure := a.Env.String("APP_ENV", "production") != "development"
		store := NewCookieStore(
			[]byte(appKey),
			WithSecure(secure),
		)
		p.cfg.Store = store
	}

	a.Register("session", p.cfg.Store)
	slog.Info("✓ Session store registered")
	return nil
}

// Boot returns the HTTP middleware for use in the router.
// Astra's HTTP server calls Boot to retrieve middleware after all providers register.
func (p *Provider) Boot(a *core.App) error {
	return nil
}

// Middleware returns the session HTTP middleware. Must be called after Boot.
func (p *Provider) Middleware() func(http.Handler) http.Handler {
	return Middleware(p.cfg.Store)
}

// Ensure os and time are used.
var (
	_ = os.Getenv
	_ = time.Second
)
