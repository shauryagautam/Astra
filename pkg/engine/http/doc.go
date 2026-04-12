// Package http provides the web layer for the Astra framework, integrating
// routing, middleware, and a high-performance request context.
//
// The package is built on top of go-chi but provides a much richer Context
// object that simplifies binding, validation, and response handling.
//
// Key Components:
//   - Router: A fluent routing engine with support for groups and versioning.
//   - Context: The central piece of a request's lifecycle. Wraps http.Request/Response.
//   - Middleware: A suite of built-in middleware for security, logging, recovery, and more.
//   - SSR: Components for Server-Side Rendering, including flash messages and asset helpers.
//
// Example:
//
//	router.Get("/users/{id}", func(c *Context) error {
//	    return c.JSON(map[string]string{"id": c.Param("id")})
//	})
package http
