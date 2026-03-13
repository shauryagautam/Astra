package http

import (
	"context"
	"sync"
)

// MiddlewareChain provides ultra-fast middleware execution with pooling
type MiddlewareChain struct {
	middleware []MiddlewareFunc
	pool       sync.Pool
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(middleware ...MiddlewareFunc) *MiddlewareChain {
	return &MiddlewareChain{
		middleware: middleware,
		pool: sync.Pool{
			New: func() any {
				return &chainExecutor{
					middleware: make([]MiddlewareFunc, 0, 10),
				}
			},
		},
	}
}

// Use adds middleware to the chain
func (mc *MiddlewareChain) Use(middleware ...MiddlewareFunc) {
	mc.middleware = append(mc.middleware, middleware...)
}

// Execute executes the middleware chain with ultra-fast pooling
func (mc *MiddlewareChain) Execute(ctx context.Context, handler HandlerFunc, c *Context) error {
	executor := mc.pool.Get().(*chainExecutor)
	defer mc.pool.Put(executor)

	return executor.execute(ctx, mc.middleware, handler, c)
}

// chainExecutor is a reusable executor for middleware chains
type chainExecutor struct {
	middleware []MiddlewareFunc
	index      int
}

func (e *chainExecutor) execute(ctx context.Context, middleware []MiddlewareFunc, handler HandlerFunc, c *Context) error {
	// Reset executor state
	e.middleware = middleware[:0]
	e.middleware = append(e.middleware, middleware...)
	e.index = 0

	return e.next(ctx, handler, c)
}

func (e *chainExecutor) next(ctx context.Context, handler HandlerFunc, c *Context) error {
	if e.index >= len(e.middleware) {
		return handler(c)
	}

	mw := e.middleware[e.index]
	e.index++

	return mw(func(c *Context) error {
		return e.next(ctx, handler, c)
	})(c)
}

// FastChain provides an ultra-fast middleware chain without pooling (for simple cases)
func FastChain(middleware []MiddlewareFunc, handler HandlerFunc) HandlerFunc {
	return func(c *Context) error {
		// Build the chain in reverse order for maximum efficiency
		final := handler
		for i := len(middleware) - 1; i >= 0; i-- {
			final = middleware[i](func(next HandlerFunc) HandlerFunc {
				return func(c *Context) error {
					return next(c)
				}
			}(final))
		}
		return final(c)
	}
}

// ChainBuilder provides a fluent interface for building middleware chains
type ChainBuilder struct {
	middleware []MiddlewareFunc
}

// NewChainBuilder creates a new chain builder
func NewChainBuilder() *ChainBuilder {
	return &ChainBuilder{
		middleware: make([]MiddlewareFunc, 0, 5),
	}
}

// Add adds middleware to the chain
func (cb *ChainBuilder) Add(mw MiddlewareFunc) *ChainBuilder {
	cb.middleware = append(cb.middleware, mw)
	return cb
}

// Build builds the final handler
func (cb *ChainBuilder) Build(handler HandlerFunc) HandlerFunc {
	return FastChain(cb.middleware, handler)
}

// Conditional adds conditional middleware
func (cb *ChainBuilder) Conditional(condition func(*Context) bool, mw MiddlewareFunc) *ChainBuilder {
	return cb.Add(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			if condition(c) {
				return mw(next)(c)
			}
			return next(c)
		}
	})
}

// Group adds multiple middleware at once
func (cb *ChainBuilder) Group(middleware ...MiddlewareFunc) *ChainBuilder {
	cb.middleware = append(cb.middleware, middleware...)
	return cb
}
