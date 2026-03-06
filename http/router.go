// Package http provides the HTTP routing and request handling layer for Astra.
// It wraps go-chi/chi with an ergonomic, error-returning handler pattern.
package http

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/astraframework/astra/core"
	"github.com/go-chi/chi/v5"
)

// HandlerFunc is Astra's handler signature. Handlers return errors,
// which are caught by the error handling middleware.
type HandlerFunc func(c *Context) error

// MiddlewareFunc is Astra's middleware signature.
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// Resource routes are bound automatically based on which standard
// methods the given controller struct implements: Index, Store, Show, Update, Destroy.
// The controller can be any struct.
type (
	hasIndex   interface{ Index(c *Context) error }
	hasStore   interface{ Store(c *Context) error }
	hasShow    interface{ Show(c *Context) error }
	hasUpdate  interface{ Update(c *Context) error }
	hasDestroy interface{ Destroy(c *Context) error }
)

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
	r := &Router{
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
	
	// Apply global security middleware (SecureHeaders)
	r.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			c.SetHeader("X-Content-Type-Options", "nosniff")
			c.SetHeader("X-Frame-Options", "DENY")
			c.SetHeader("X-XSS-Protection", "1; mode=block")
			c.SetHeader("Referrer-Policy", "strict-origin-when-cross-origin")
			c.SetHeader("Content-Security-Policy", "default-src 'self'")
			return next(c)
		}
	})
	r.Use(func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			var limit int64 = 10 * 1024 * 1024 // 10MB default
			if c.App != nil && c.App.Config != nil && c.App.Config.App.MaxBodySize > 0 {
				limit = c.App.Config.App.MaxBodySize
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
			return next(c)
		}
	})

	return r
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
// It detects which standard methods the controller implements (Index, Store, Show, Update, Destroy)
// and maps them automatically.
//
// Detects:
//
//	GET    /pattern          → controller.Index
//	POST   /pattern          → controller.Store
//	GET    /pattern/{id}     → controller.Show
//	PUT    /pattern/{id}     → controller.Update
//	PATCH  /pattern/{id}     → controller.Update
//	DELETE /pattern/{id}     → controller.Destroy
func (r *Router) Resource(pattern string, controller any) {
	pattern = "/" + strings.Trim(pattern, "/")
	
	if ctrl, ok := controller.(hasIndex); ok {
		r.Get(pattern, ctrl.Index)
	}
	if ctrl, ok := controller.(hasStore); ok {
		r.Post(pattern, ctrl.Store)
	}
	if ctrl, ok := controller.(hasShow); ok {
		r.Get(pattern+"/{id}", ctrl.Show)
	}
	if ctrl, ok := controller.(hasUpdate); ok {
		r.Put(pattern+"/{id}", ctrl.Update)
		r.Patch(pattern+"/{id}", ctrl.Update)
	}
	if ctrl, ok := controller.(hasDestroy); ok {
		r.Delete(pattern+"/{id}", ctrl.Destroy)
	}
}

// Mount attaches a sub-router at the given pattern.
func (r *Router) Mount(pattern string, handler http.Handler) {
	r.mux.Mount(pattern, handler)
}

// Static registers a route to serve static files from a directory.
func (r *Router) Static(path string, root string) {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	r.mux.Handle(path+"*", http.StripPrefix(path, http.FileServer(http.Dir(root))))
}

// MountSwagger mounts the Swagger UI and OpenAPI JSON endpoints.
func (r *Router) MountSwagger() {
	if os.Getenv("APP_ENV") != "development" {
		return
	}
	// Serve OpenAPI spec (assuming it's at docs/openapi.json)
	r.mux.Get("/openapi.json", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, "docs/openapi.json")
	})
	// Serve Swagger UI (could be a CDN or local static files)
	r.mux.Get("/docs", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head><link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@3/swagger-ui.css" ><script src="https://unpkg.com/swagger-ui-dist@3/swagger-ui-bundle.js"> </script></head><body><div id="swagger-ui"></div><script>window.onload = function() { SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui" }) }</script></body></html>`))
	})
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
