// Package http provides the HTTP routing and request handling layer for Astra.
// It wraps go-chi/chi with an ergonomic, error-returning handler pattern.
package http

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/astraframework/astra/core"
	"github.com/go-chi/chi/v5"
)

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
	mux           chi.Router
	App           *core.App
	errorHandler  func(c *Context, err error)
	middleware    []MiddlewareFunc
	beforeHooks   []HandlerFunc
	afterHooks    []HandlerFunc
	namedRoutes   map[string]string
	prefix        string
	templateFuncs map[string]any
}

// Route represents a registered route.
type Route struct {
	Method  string
	Pattern string
	router  *Router
}

// Name assigns a name to the route for reverse URL generation.
func (r *Route) Name(name string) *Route {
	if r.router.namedRoutes == nil {
		r.router.namedRoutes = make(map[string]string)
	}
	r.router.namedRoutes[name] = r.Pattern
	return r
}

// NewRouter creates a new Router backed by chi.
func NewRouter(app *core.App) *Router {
	r := &Router{
		mux:           chi.NewRouter(),
		App:           app,
		middleware:    make([]MiddlewareFunc, 0),
		beforeHooks:   make([]HandlerFunc, 0),
		afterHooks:    make([]HandlerFunc, 0),
		namedRoutes:   make(map[string]string),
		prefix:        "",
		errorHandler:  NewInteractiveErrorHandler(app).Handle,
		templateFuncs: make(map[string]any),
	}

	// Register default template helpers
	r.registerDefaultHelpers()

	// Register default middleware stack in order
	if app != nil && app.Logger != nil {
		r.Use(Recover(app.Logger))
	}
	r.Use(RequestID())
	if app != nil && app.Logger != nil {
		r.Use(Logger(app.Logger))
	}
	r.Use(SecureHeaders())
	r.Use(OpenTelemetry())
	r.Use(MaxBodySize(0)) // 0 means use config or default 10MB

	// Telemetry (Metrics, Tracing, Health) are now registered by their
	// respective providers during the Boot phase to avoid import cycles.

	// Auto-register dashboard if available and NOT in production

	// Auto-register dashboard if available and NOT in production
	if app != nil && !app.Env.IsProd() {
		if dashSvc := app.Get("dashboard"); dashSvc != nil {
			if dash, ok := dashSvc.(*core.Dashboard); ok {
				RegisterDashboardRoutes(r, dash)
				r.Use(DashboardTracker(dash))
			}
		}
	}

	return r
}

func defaultErrorHandler(c *Context, err error) {
	// Default error handler — overridden by user or ErrorHandler middleware
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "An unexpected error occurred"

	if he, ok := err.(*HTTPError); ok {
		status = he.Status
		code = he.Code
		message = he.Message
	}

	if err := c.JSON(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}, status); err != nil {
		// Ignore JSON error
	}
}

// MaxBodySize returns a middleware that limits the request body size.
func MaxBodySize(limit int64) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			l := limit
			if l <= 0 {
				l = 10 * 1024 * 1024 // 10MB default
				if c.App != nil && c.App.Config != nil && c.App.Config.App.MaxBodySize > 0 {
					l = c.App.Config.App.MaxBodySize
				}
			}

			// Allow per-route override via context
			if override := c.Get("max_body_size"); override != nil {
				if ol, ok := override.(int64); ok {
					l = ol
				}
			}

			// Fast path: Reject immediately if the client advertises a payload that is too large
			if c.Request.ContentLength > l {
				return NewHTTPError(http.StatusRequestEntityTooLarge, "ENTITY_TOO_LARGE", fmt.Sprintf("Request body exceeds limit of %d bytes", l))
			}

			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, l)
			err := next(c)

			// Catch "request body too large" error from net/http
			if err != nil && strings.Contains(err.Error(), "request body too large") {
				return NewHTTPError(http.StatusRequestEntityTooLarge, "ENTITY_TOO_LARGE", fmt.Sprintf("Request body exceeds limit of %d bytes", l))
			}
			return err
		}
	}
}

// DashboardTracker returns a middleware that tracks requests in the dev dashboard.
func DashboardTracker(dash *core.Dashboard) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)

			// Don't track dashboard's own API calls to avoid noise
			if !strings.HasPrefix(c.Request.URL.Path, "/__astra") {
				dash.TrackRequest(c.Request.Method, c.Request.URL.Path, c.Status(), duration)
			}
			return err
		}
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
	return &Route{Method: "GET", Pattern: joinRoutePattern(r.prefix, pattern), router: r}
}

// Post registers a POST route.
func (r *Router) Post(pattern string, handler HandlerFunc) *Route {
	r.mux.Post(pattern, r.wrap(handler))
	return &Route{Method: "POST", Pattern: joinRoutePattern(r.prefix, pattern), router: r}
}

// Put registers a PUT route.
func (r *Router) Put(pattern string, handler HandlerFunc) *Route {
	r.mux.Put(pattern, r.wrap(handler))
	return &Route{Method: "PUT", Pattern: joinRoutePattern(r.prefix, pattern), router: r}
}

// Patch registers a PATCH route.
func (r *Router) Patch(pattern string, handler HandlerFunc) *Route {
	r.mux.Patch(pattern, r.wrap(handler))
	return &Route{Method: "PATCH", Pattern: joinRoutePattern(r.prefix, pattern), router: r}
}

// Delete registers a DELETE route.
func (r *Router) Delete(pattern string, handler HandlerFunc) *Route {
	r.mux.Delete(pattern, r.wrap(handler))
	return &Route{Method: "DELETE", Pattern: joinRoutePattern(r.prefix, pattern), router: r}
}

// Options registers an OPTIONS route.
func (r *Router) Options(pattern string, handler HandlerFunc) *Route {
	r.mux.Options(pattern, r.wrap(handler))
	return &Route{Method: "OPTIONS", Pattern: joinRoutePattern(r.prefix, pattern), router: r}
}

// Head registers a HEAD route.
func (r *Router) Head(pattern string, handler HandlerFunc) *Route {
	r.mux.Head(pattern, r.wrap(handler))
	return &Route{Method: "HEAD", Pattern: joinRoutePattern(r.prefix, pattern), router: r}
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
			middleware:   append([]MiddlewareFunc(nil), r.middleware...),
			beforeHooks:  append([]HandlerFunc(nil), r.beforeHooks...),
			afterHooks:   append([]HandlerFunc(nil), r.afterHooks...),
			namedRoutes:  r.namedRoutes,
			prefix:       joinRoutePattern(r.prefix, pattern),
		}
		fn(sub)
	})
}

// V1 creates a route group prefixed with /v1.
func (r *Router) V1(fn func(r *Router)) {
	r.Group("/v1", fn)
}

// V2 creates a route group prefixed with /v2.
func (r *Router) V2(fn func(r *Router)) {
	r.Group("/v2", fn)
}

// VN creates a route group prefixed with /v[n].
func (r *Router) VN(n int, fn func(r *Router)) {
	r.Group(fmt.Sprintf("/v%d", n), fn)
}

// AcceptVersion returns a middleware that checks for a specific API version in the Accept header.
// Format: Accept: application/vnd.astra.v[version]+json
func AcceptVersion(version string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			accept := c.Request.Header.Get("Accept")
			if accept == "" {
				return next(c)
			}

			expected := fmt.Sprintf("application/vnd.astra.v%s+json", strings.TrimLeft(version, "v"))
			if !strings.Contains(accept, expected) {
				// We don't block yet, just let the router try to match.
				// But we can mark the version for later use.
				c.Set("api_version", version)
			}
			return next(c)
		}
	}
}

func joinRoutePattern(prefix string, pattern string) string {
	if prefix == "" {
		if pattern == "" {
			return "/"
		}
		if strings.HasPrefix(pattern, "/") {
			return pattern
		}
		return "/" + pattern
	}
	if pattern == "" || pattern == "/" {
		return prefix
	}
	return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(pattern, "/")
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
		if _, err := w.Write([]byte(`<!DOCTYPE html><html><head><link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@3/swagger-ui.css" ><script src="https://unpkg.com/swagger-ui-dist@3/swagger-ui-bundle.js"> </script></head><body><div id="swagger-ui"></div><script>window.onload = function() { SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui" }) }</script></body></html>`)); err != nil {
			// Ignore write error
		}
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
	// Pre-compute middleware chain for this route
	h := handler
	for i := len(r.middleware) - 1; i >= 0; i-- {
		h = r.middleware[i](h)
	}

	// Capture hooks and error handler locally to avoid pointer dereferences on hot path
	before := r.beforeHooks
	after := r.afterHooks
	errH := r.errorHandler
	app := r.App

	return func(w http.ResponseWriter, req *http.Request) {
		c := NewContext(w, req, app)
		defer c.release()

		// Run before-hooks first
		for i := 0; i < len(before); i++ {
			if err := before[i](c); err != nil {
				errH(c, err)
				return
			}
		}

		// Run pre-computed handler chain (includes middleware)
		if err := h(c); err != nil {
			errH(c, err)
		}

		// Run after-hooks (always, regardless of error)
		for i := 0; i < len(after); i++ {
			_ = after[i](c)
		}
	}
}

func (r *Router) registerDefaultHelpers() {
	r.templateFuncs["asset_path"] = func(name string) string {
		// In a real implementation, this would look up a manifest.json
		// For now, return a simple /assets/name path
		return "/assets/" + name
	}

	r.templateFuncs["asset_tag"] = func(name string) string {
		path := "/assets/" + name
		if strings.HasSuffix(name, ".css") {
			return fmt.Sprintf(`<link rel="stylesheet" href="%s">`, path)
		}
		if strings.HasSuffix(name, ".js") {
			return fmt.Sprintf(`<script src="%s"></script>`, path)
		}
		return ""
	}

	r.templateFuncs["csrf_token"] = func() string {
		return "CSRF_TOKEN_PLACEHOLDER"
	}

	r.templateFuncs["csrf_field"] = func() template.HTML {
		return template.HTML(fmt.Sprintf(`<input type="hidden" name="_csrf" value="%s">`, "CSRF_TOKEN_PLACEHOLDER"))
	}

	r.templateFuncs["route"] = func(name string, params ...any) string {
		return r.Route(name, params...)
	}
}

// AddFunc adds a global template function.
func (r *Router) AddFunc(name string, fn any) {
	r.templateFuncs[name] = fn
}

// Funcs returns the global template function map.
func (r *Router) Funcs() map[string]any {
	return r.templateFuncs
}
