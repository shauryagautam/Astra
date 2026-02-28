package contracts

import (
	"context"
	"net/http"
)

// HandlerFunc is the signature for route handlers.
// Receives the HttpContext and returns an error (nil on success).
// Mirrors AdonisJS: async ({ request, response, auth }) => {}
type HandlerFunc func(ctx HttpContextContract) error

// MiddlewareFunc is the signature for middleware functions.
// Receives the context and a next function to call the next middleware.
// Mirrors AdonisJS: async (ctx, next) => { await next() }
type MiddlewareFunc func(ctx HttpContextContract, next func() error) error

// HttpContextContract is the single object passed to every route handler
// and middleware. It wraps the HTTP request/response and provides access
// to all framework features. Replicates AdonisJS's HttpContextContract.
//
// In AdonisJS: const { request, response, auth, params } = ctx
// In Go:       req := ctx.Request(); resp := ctx.Response()
type HttpContextContract interface {
	// Request returns the wrapped HTTP request object.
	Request() RequestContract

	// Response returns the wrapped HTTP response object.
	Response() ResponseContract

	// Params returns route parameters as a map.
	// e.g., for route "/users/:id", ctx.Params()["id"] returns the value.
	Params() map[string]string

	// Param returns a single route parameter by name.
	Param(key string) string

	// Logger returns a contextual logger.
	Logger() any

	// Auth returns the auth guard for the current request.
	// Returns nil if auth is not configured.
	Auth() any

	// GetRaw returns the underlying *http.Request.
	GetRaw() *http.Request

	// GetResponseWriter returns the underlying http.ResponseWriter.
	GetResponseWriter() http.ResponseWriter

	// Context returns the Go standard context.Context.
	Context() context.Context

	// WithValue stores a value in the context for the current request.
	WithValue(key string, value any)

	// GetValue retrieves a value from the context.
	GetValue(key string) any
}

// RequestContract wraps the incoming HTTP request with convenience methods.
// Mirrors AdonisJS's Request class.
type RequestContract interface {
	// Method returns the HTTP method (GET, POST, etc.).
	Method() string

	// URL returns the full request URL path.
	URL() string

	// Header returns the value of a request header.
	Header(key string) string

	// Headers returns all request headers.
	Headers() http.Header

	// Input returns a single input value by key from body or query string.
	// Mirrors: request.input('key')
	Input(key string) string

	// InputOr returns input value or a default if not present.
	// Mirrors: request.input('key', defaultValue)
	InputOr(key string, defaultValue string) string

	// All returns all input data (merged query + body) as a map.
	// Mirrors: request.all()
	All() map[string]any

	// Only returns only the specified keys from input.
	// Mirrors: request.only(['key1', 'key2'])
	Only(keys ...string) map[string]any

	// Except returns all input except the specified keys.
	// Mirrors: request.except(['key1'])
	Except(keys ...string) map[string]any

	// QueryString returns query parameters as a map.
	Qs() map[string]string

	// Cookie returns a cookie value by name.
	Cookie(name string) string

	// HasBody returns true if the request has a body.
	HasBody() bool

	// IP returns the client IP address.
	IP() string

	// IsAjax returns true if the request is an AJAX request.
	IsAjax() bool

	// Raw returns the underlying *http.Request.
	Raw() *http.Request

	// Body reads and returns the raw request body as bytes.
	Body() ([]byte, error)
}

// ResponseContract wraps the HTTP response writer with convenience methods.
// Mirrors AdonisJS's Response class.
type ResponseContract interface {
	// Status sets the HTTP status code for the response.
	// Returns the response for chaining.
	// Mirrors: response.status(200)
	Status(code int) ResponseContract

	// Json sends a JSON response with appropriate content-type.
	// Mirrors: response.json({ key: 'value' })
	Json(data any) error

	// Send sends a plain text response.
	// Mirrors: response.send('text')
	Send(data string) error

	// SendBytes sends raw bytes.
	SendBytes(data []byte) error

	// Header sets a response header.
	// Mirrors: response.header('key', 'value')
	Header(key string, value string) ResponseContract

	// Cookie sets a cookie on the response.
	Cookie(name string, value string, options ...CookieOption) ResponseContract

	// ClearCookie removes a cookie.
	ClearCookie(name string) ResponseContract

	// Redirect sends a redirect response.
	// Mirrors: response.redirect('/path')
	Redirect(url string) error

	// RedirectStatus sends a redirect with a specific status code.
	RedirectStatus(url string, code int) error

	// Abort sends an error response and stops the handler chain.
	Abort(code int, message string) error

	// NoContent sends a 204 No Content response.
	NoContent() error

	// Created sends a 201 Created response with JSON body.
	Created(data any) error

	// GetStatusCode returns the current response status code.
	GetStatusCode() int

	// IsCommitted returns true if headers have been sent.
	IsCommitted() bool

	// Raw returns the underlying http.ResponseWriter.
	Raw() http.ResponseWriter
}

// CookieOption configures cookie attributes.
type CookieOption struct {
	MaxAge   int
	Path     string
	Domain   string
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
}

// RouteContract represents a single registered route.
type RouteContract interface {
	// Pattern returns the URL pattern (e.g., "/users/:id").
	Pattern() string

	// Methods returns the HTTP methods this route handles.
	Methods() []string

	// Handler returns the handler function.
	Handler() HandlerFunc

	// Name returns the route name.
	Name() string

	// As sets the route name. Returns the route for chaining.
	// Mirrors: Route.get('/users', handler).as('users.index')
	As(name string) RouteContract

	// Middleware attaches named middleware to the route.
	// Mirrors: Route.get('/users', handler).middleware('auth')
	Middleware(names ...string) RouteContract

	// GetMiddleware returns the middleware names attached to this route.
	GetMiddleware() []string

	// Prefix returns the route prefix.
	Prefix() string
}

// RouteGroupContract represents a group of routes sharing common config.
type RouteGroupContract interface {
	// Prefix sets the URL prefix for the group.
	// Mirrors: Route.group(() => {}).prefix('/api')
	Prefix(prefix string) RouteGroupContract

	// Middleware attaches middleware to all routes in the group.
	// Mirrors: Route.group(() => {}).middleware(['auth'])
	Middleware(names ...string) RouteGroupContract

	// Namespace sets the controller namespace for the group.
	Namespace(namespace string) RouteGroupContract

	// As sets a name prefix for all routes in the group.
	As(name string) RouteGroupContract
}

// ResourceController defines the interface for RESTful resource controllers.
// Mirrors AdonisJS's resource controller methods.
type ResourceController interface {
	Index(ctx HttpContextContract) error
	Store(ctx HttpContextContract) error
	Show(ctx HttpContextContract) error
	Update(ctx HttpContextContract) error
	Destroy(ctx HttpContextContract) error
}

// RouterContract defines the routing API.
// Mirrors AdonisJS's Route module with its fluid API.
type RouterContract interface {
	// Get registers a GET route.
	// Mirrors: Route.get('/path', handler)
	Get(pattern string, handler HandlerFunc) RouteContract

	// Post registers a POST route.
	Post(pattern string, handler HandlerFunc) RouteContract

	// Put registers a PUT route.
	Put(pattern string, handler HandlerFunc) RouteContract

	// Patch registers a PATCH route.
	Patch(pattern string, handler HandlerFunc) RouteContract

	// Delete registers a DELETE route.
	Delete(pattern string, handler HandlerFunc) RouteContract

	// Any registers a route for all HTTP methods.
	Any(pattern string, handler HandlerFunc) RouteContract

	// Group creates a route group with shared configuration.
	// Mirrors: Route.group(() => { Route.get(...) }).prefix('/api')
	Group(callback func(group RouterContract)) RouteGroupContract

	// Resource registers RESTful resource routes.
	// Mirrors: Route.resource('users', UsersController)
	Resource(name string, controller ResourceController) RouteGroupContract

	// Middleware returns all registered middleware.
	GetRoutes() []RouteContract

	// FindRoute resolves a route for the given method and path.
	FindRoute(method string, path string) (RouteContract, map[string]string, bool)

	// Commit finalizes route registration and compiles the route tree.
	Commit()
}

// ServerContract defines the HTTP server interface.
type ServerContract interface {
	// SetRouter sets the router for the server.
	SetRouter(router RouterContract)

	// Use registers a global middleware.
	// Mirrors: Server.middleware.register([...])
	Use(middleware ...MiddlewareFunc)

	// RegisterNamed registers named middleware that can be referenced by routes.
	// Mirrors: Server.middleware.registerNamed({ auth: AuthMiddleware })
	RegisterNamed(name string, middleware MiddlewareFunc)

	// Start begins listening for HTTP requests.
	Start(addr string) error

	// Shutdown gracefully stops the server.
	Shutdown(ctx context.Context) error
}
