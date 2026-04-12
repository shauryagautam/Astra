package http

import (
	"net/http"
)

// Chain stacks standard middleware around an http.Handler.
func Chain(mws []MiddlewareFunc, h http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// ChainBuilder provides a fluent interface for building standard middleware chains.
type ChainBuilder struct {
	middleware []MiddlewareFunc
}

// NewChainBuilder creates a new chain builder.
func NewChainBuilder() *ChainBuilder {
	return &ChainBuilder{
		middleware: make([]MiddlewareFunc, 0, 8),
	}
}

// Add adds standard middleware to the chain.
func (cb *ChainBuilder) Add(mw MiddlewareFunc) *ChainBuilder {
	cb.middleware = append(cb.middleware, mw)
	return cb
}

// Build wraps the final handler with the middleware chain.
func (cb *ChainBuilder) Build(handler http.Handler) http.Handler {
	return Chain(cb.middleware, handler)
}

// HandleContext wraps an Astra HandlerFunc with the middleware chain.
func (cb *ChainBuilder) HandleContext(h HandlerFunc) http.Handler {
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := NewContext(w, r)
		defer c.release()
		if err := h(c); err != nil {
			// Basic error handling if not already handled
			if !c.written {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	})
	return cb.Build(final)
}
