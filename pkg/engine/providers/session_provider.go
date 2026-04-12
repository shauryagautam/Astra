package providers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/session"
)

type SessionProvider struct {
	engine.BaseProvider
	store session.Store
}

type sessionWrapper struct {
	inner session.Store
}

func (w *sessionWrapper) Load(r *http.Request) (any, error) {
	return w.inner.Load(r)
}

func (w *sessionWrapper) Save(rw http.ResponseWriter, s any) error {
	sess, ok := s.(*session.Session)
	if !ok {
		return fmt.Errorf("invalid session type")
	}
	return w.inner.Save(rw, sess)
}

func NewSessionProvider(store session.Store) *SessionProvider {
	return &SessionProvider{store: store}
}

func (p *SessionProvider) Name() string { return "session" }

func (p *SessionProvider) Register(a *engine.App) error {
	if p.store == nil {
		appKey := a.Env().String("APP_KEY", "")
		if appKey == "" {
			return fmt.Errorf("session: APP_KEY is not set")
		}
		p.store = session.NewCookieStore([]byte(appKey))
	}
	slog.Info("session store initialized")
	return nil
}

