package engine

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/shauryagautam/Astra/pkg/mail"
)

// HTTPRouter defines the minimal interface for the application router.
type HTTPRouter interface {
	http.Handler
}

// ViewEngine defines the interface for template rendering.
// This is defined here to avoid circular dependency with the http package.
type ViewEngine interface {
	Render(wr io.Writer, name string, data any) error
}


// i18n
type Translator interface {
	T(locale, key string, args ...any) string
}

// mail
type Mailer interface {
	Send(ctx context.Context, msg *mail.Message) error
}

// notification
type Notifier interface {
	Notify(ctx context.Context, n any) error
}

// storage
type Storage interface {
	Put(ctx context.Context, path string, data []byte) error
	Get(ctx context.Context, path string) ([]byte, error)
}

// session
type SessionStore interface {
	Load(r *http.Request) (any, error)
	Save(w http.ResponseWriter, s any) error
}

type Session interface {
	Get(key string) any
	Set(key string, value any)
	Save(w http.ResponseWriter) error
}

// cache
type CacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
}

// validate
type Validator interface {
	Validate(any) error
	BindAndValidate(r *http.Request, v any) error
}

// HealthProvider defines the interface for components that can be health-checked.
type HealthProvider interface {
	CheckHealth(ctx context.Context) error
}

// HealthCheckFunc is a function type that implements HealthProvider.
type HealthCheckFunc func(context.Context) error

func (f HealthCheckFunc) CheckHealth(ctx context.Context) error {
	return f(ctx)
}
