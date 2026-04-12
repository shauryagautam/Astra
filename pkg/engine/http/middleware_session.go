package http

import (
	"context"
	stdhttp "net/http"

	"github.com/shauryagautam/Astra/pkg/session"
)

// SessionContextKey is the key used to store the session in the Astra context.
const SessionContextKey = "astra.session"

// SessionMiddleware returns a standard middleware that loads the session from the
// request and stores it in the request context.
func SessionMiddleware(store session.Store) MiddlewareFunc {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			sess, err := store.Load(r)
			if err != nil {
				// We don't block request on session load error, usually.
				// But we should at least have a session object.
				next.ServeHTTP(w, r)
				return
			}

			// Inject into request context for Astra Context to pick up
			ctx := context.WithValue(r.Context(), SessionContextKey, sess)
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

// ─── savingResponseWriter ──────────────────────────────────────────────────────

// savingResponseWriter wraps ResponseWriter to save the session before
// headers are written.
type savingResponseWriter struct {
	stdhttp.ResponseWriter
	sess  *session.Session
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

func (sw *savingResponseWriter) saveOnce(w stdhttp.ResponseWriter) {
	if sw.saved {
		return
	}
	sw.saved = true
	if sw.sess != nil {
		_ = sw.sess.Save(w)
	}
}

// Unwrap allows middleware that needs the underlying ResponseWriter to access it.
func (sw *savingResponseWriter) Unwrap() stdhttp.ResponseWriter { return sw.ResponseWriter }
