// Package http provides the HTTP routing and request handling layer for Astra.
// It wraps go-chi/chi with an ergonomic, error-returning handler pattern.
package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/astraframework/astra/core"
	"github.com/go-chi/chi/v5"
)

// HandlerFunc is Astra's handler signature. Handlers return errors,
// which are caught by the error handling middleware.
type HandlerFunc func(c *Context) error

// MiddlewareFunc is Astra's middleware signature.
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// ResourceController defines the interface for RESTful resource controllers.
type ResourceController interface {
	Index(c *Context) error
	Store(c *Context) error
	Show(c *Context) error
	Update(c *Context) error
	Destroy(c *Context) error
}

// Router wraps chi.Router with Astra's error-returning handler pattern
// and convenience methods for route groups and resource routes.
type Router struct {
	mux          chi.Router
	App          *core.App
	errorHandler func(c *Context, err error)
	middleware   []MiddlewareFunc
	beforeHooks  []HandlerFunc
	afterHooks   []HandlerFunc
	namedRoutes  map[string]string
}

// Route represents a registered route.
type Route struct {
	Method  string
	Pattern string
	router  *Router
}

// Name assigns a name to the route for reverse URL generation.
func (r *Route) Name(name string) *Route {
	r.router.namedRoutes[name] = r.Pattern
	return r
}

// NewRouter creates a new Router backed by chi.
func NewRouter(app *core.App) *Router {
	return &Router{
		mux:         chi.NewRouter(),
		App:         app,
		middleware:  make([]MiddlewareFunc, 0),
		beforeHooks: make([]HandlerFunc, 0),
		afterHooks:  make([]HandlerFunc, 0),
		namedRoutes: make(map[string]string),
		errorHandler: func(c *Context, err error) {
			// Default error handler — overridden by user or ErrorHandler middleware
			status := http.StatusInternalServerError
			code := "INTERNAL_ERROR"
			message := "An unexpected error occurred"

			if he, ok := err.(*HTTPError); ok {
				status = he.Status
				code = he.Code
				message = he.Message
			}

			c.JSON(map[string]any{
				"error": map[string]any{
					"code":    code,
					"message": message,
				},
			}, status)
		},
	}
}

// SetErrorHandler sets a custom error handler for the router.
func (r *Router) SetErrorHandler(fn func(c *Context, err error)) {
	r.errorHandler = fn
}

// Before registers a hook that runs before every request handler.
// If a before-hook returns an error, the handler is not called.
// Useful for: setting tenant from subdomain, injecting request-scoped data.
func (r *Router) Before(fn HandlerFunc) {
	r.beforeHooks = append(r.beforeHooks, fn)
}

// After registers a hook that runs after every request handler.
// After-hooks always run, even if the handler returned an error.
// Useful for: audit logging, response tracking.
func (r *Router) After(fn HandlerFunc) {
	r.afterHooks = append(r.afterHooks, fn)
}

// Use registers global middleware. Middleware is applied in order.
func (r *Router) Use(middleware ...MiddlewareFunc) {
	r.middleware = append(r.middleware, middleware...)
}

// UseStd registers standard net/http middleware (for chi compatibility).
func (r *Router) UseStd(middleware ...func(http.Handler) http.Handler) {
	for _, mw := range middleware {
		r.mux.Use(mw)
	}
}

// Get registers a GET route.
func (r *Router) Get(pattern string, handler HandlerFunc) *Route {
	r.mux.Get(pattern, r.wrap(handler))
	return &Route{Method: "GET", Pattern: pattern, router: r}
}

// Post registers a POST route.
func (r *Router) Post(pattern string, handler HandlerFunc) *Route {
	r.mux.Post(pattern, r.wrap(handler))
	return &Route{Method: "POST", Pattern: pattern, router: r}
}

// Put registers a PUT route.
func (r *Router) Put(pattern string, handler HandlerFunc) *Route {
	r.mux.Put(pattern, r.wrap(handler))
	return &Route{Method: "PUT", Pattern: pattern, router: r}
}

// Patch registers a PATCH route.
func (r *Router) Patch(pattern string, handler HandlerFunc) *Route {
	r.mux.Patch(pattern, r.wrap(handler))
	return &Route{Method: "PATCH", Pattern: pattern, router: r}
}

// Delete registers a DELETE route.
func (r *Router) Delete(pattern string, handler HandlerFunc) *Route {
	r.mux.Delete(pattern, r.wrap(handler))
	return &Route{Method: "DELETE", Pattern: pattern, router: r}
}

// Options registers an OPTIONS route.
func (r *Router) Options(pattern string, handler HandlerFunc) *Route {
	r.mux.Options(pattern, r.wrap(handler))
	return &Route{Method: "OPTIONS", Pattern: pattern, router: r}
}

// Head registers a HEAD route.
func (r *Router) Head(pattern string, handler HandlerFunc) *Route {
	r.mux.Head(pattern, r.wrap(handler))
	return &Route{Method: "HEAD", Pattern: pattern, router: r}
}

// Route returns the URL pattern for a named route.
func (r *Router) Route(name string, params ...any) string {
	pattern, ok := r.namedRoutes[name]
	if !ok {
		return ""
	}

	for _, p := range params {
		// Basic replacement of first placeholder found
		start := strings.Index(pattern, "{")
		end := strings.Index(pattern, "}")
		if start != -1 && end != -1 {
			pattern = pattern[:start] + fmt.Sprintf("%v", p) + pattern[end+1:]
		}
	}
	return pattern
}

// Group creates a route group with a shared prefix and optional middleware.
func (r *Router) Group(pattern string, fn func(r *Router)) {
	r.mux.Route(pattern, func(cr chi.Router) {
		sub := &Router{
			mux:          cr,
			App:          r.App,
			errorHandler: r.errorHandler,
			middleware:   make([]MiddlewareFunc, len(r.middleware)),
		}
		copy(sub.middleware, r.middleware)
		fn(sub)
	})
}

// Resource registers RESTful resource routes for a controller.
//
// Generates:
//
//	GET    /pattern          → controller.Index
//	POST   /pattern          → controller.Store
//	GET    /pattern/{id}     → controller.Show
//	PUT    /pattern/{id}     → controller.Update
//	DELETE /pattern/{id}     → controller.Destroy
func (r *Router) Resource(pattern string, controller ResourceController) {
	pattern = "/" + strings.Trim(pattern, "/")
	r.Get(pattern, controller.Index)
	r.Post(pattern, controller.Store)
	r.Get(pattern+"/{id}", controller.Show)
	r.Put(pattern+"/{id}", controller.Update)
	r.Patch(pattern+"/{id}", controller.Update)
	r.Delete(pattern+"/{id}", controller.Destroy)
}

// Mount attaches a sub-router at the given pattern.
func (r *Router) Mount(pattern string, handler http.Handler) {
	r.mux.Mount(pattern, handler)
}

// ServeHTTP implements http.Handler for the router.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// Handler returns the underlying http.Handler for the router.
func (r *Router) Handler() http.Handler {
	return r.mux
}

// PrintRoutes returns a formatted table of all registered routes.
func (r *Router) PrintRoutes() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-8s %s\n", "METHOD", "PATTERN"))
	sb.WriteString(strings.Repeat("─", 60) + "\n")

	walkFn := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		sb.WriteString(fmt.Sprintf("%-8s %s\n", method, route))
		return nil
	}

	if err := chi.Walk(r.mux, walkFn); err != nil {
		sb.WriteString(fmt.Sprintf("error walking routes: %v\n", err))
	}
	return sb.String()
}

// wrap converts an Astra HandlerFunc to a standard http.HandlerFunc,
// applying the middleware chain, before/after hooks, and error handling.
func (r *Router) wrap(handler HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		c := NewContext(w, req, r.App)
		defer c.release()

		// Apply middleware chain
		h := handler
		for i := len(r.middleware) - 1; i >= 0; i-- {
			h = r.middleware[i](h)
		}

		// Run before-hooks first
		for _, hook := range r.beforeHooks {
			if err := hook(c); err != nil {
				r.errorHandler(c, err)
				return
			}
		}

		// Run handler
		handlerErr := h(c)
		if handlerErr != nil {
			r.errorHandler(c, handlerErr)
		}

		// Run after-hooks (always, regardless of error)
		for _, hook := range r.afterHooks {
			if err := hook(c); err != nil {
				// Log but don't override the original handler error
				_ = err
			}
		}
	}
}
