package http

import "net/http"

// HandlerFunc is the standard Astra request handler function.
// It returns an error to allow for centralized error handling.
type HandlerFunc func(c *Context) error

// MiddlewareFunc is a function that wraps an http.Handler.
type MiddlewareFunc func(http.Handler) http.Handler
