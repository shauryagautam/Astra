package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// ErrorMiddleware handles error recovery and reporting.
type ErrorMiddleware struct {
	logger *slog.Logger
}

func NewErrorMiddleware(logger *slog.Logger) *ErrorMiddleware {
	return &ErrorMiddleware{logger: logger}
}

func (m *ErrorMiddleware) HandleErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				m.logger.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
				)
				if rw, ok := w.(*responseWriter); ok {
					if rw.Status() == 0 {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprintf(w, "Internal Server Error")
					}
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}
