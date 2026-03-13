package session

import (
	"context"
	"net/http"
)

// ContextKey is the key used to store the session in the request context.
// Exported so that other packages (e.g. http.Context.Session()) can look up
// the session without importing this package (avoiding import cycles).
const ContextKey = "astra.session"

// Middleware returns an HTTP middleware that loads the session from the
// request and stores it in the request context.
// The session is automatically saved to the response after the handler runs
// only if it was modified (dirty flag).
//
// Usage:
//
//	router.Use(session.Middleware(store))
func Middleware(store Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := store.Load(r)
			if err != nil {
				// Log and continue with empty session — never block the request.
				sess = &Session{
					data:  make(map[string]any),
					store: store,
					name:  defaultCookieName,
					opts:  defaultCookieOptions(),
				}
			}

			// Inject session into context.
			ctx := context.WithValue(r.Context(), ContextKey, sess)
			r = r.WithContext(ctx)

			// Use a response wrapper that saves the session before headers are flushed.
			sw := &savingResponseWriter{
				ResponseWriter: w,
				sess:           sess,
				saved:          false,
			}

			next.ServeHTTP(sw, r)

			// Final save if headers weren't already written.
			sw.saveOnce(w)
		})
	}
}

// FromContext retrieves the session from the request context.
// Returns nil if no session is in context (middleware not registered).
func FromContext(ctx context.Context) *Session {
	sess, _ := ctx.Value(ContextKey).(*Session)
	return sess
}

// ─── savingResponseWriter ──────────────────────────────────────────────────────

// savingResponseWriter wraps http.ResponseWriter to save the session before
// headers are written (i.e., before WriteHeader or Write is called).
type savingResponseWriter struct {
	http.ResponseWriter
	sess  *Session
	saved bool
}

func (sw *savingResponseWriter) WriteHeader(code int) {
	sw.saveOnce(sw.ResponseWriter)
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *savingResponseWriter) Write(b []byte) (int, error) {
	sw.saveOnce(sw.ResponseWriter)
	return sw.ResponseWriter.Write(b)
}

func (sw *savingResponseWriter) saveOnce(w http.ResponseWriter) {
	if sw.saved {
		return
	}
	sw.saved = true
	if sw.sess != nil && sw.sess.dirty {
		_ = sw.sess.store.Save(w, sw.sess)
	}
}

// Unwrap allows middleware that needs the underlying ResponseWriter to access it.
func (sw *savingResponseWriter) Unwrap() http.ResponseWriter { return sw.ResponseWriter }
