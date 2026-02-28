package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/shaurya/adonis/contracts"
)

// Server is the HTTP server that ties together the router, middleware pipeline,
// and request handling. Mirrors AdonisJS's Server class.
type Server struct {
	mu               sync.RWMutex
	router           contracts.RouterContract
	globalMiddleware []contracts.MiddlewareFunc
	namedMiddleware  map[string]contracts.MiddlewareFunc
	httpServer       *http.Server
	logger           *log.Logger
}

// NewServer creates a new HTTP Server.
func NewServer() *Server {
	return &Server{
		globalMiddleware: make([]contracts.MiddlewareFunc, 0),
		namedMiddleware:  make(map[string]contracts.MiddlewareFunc),
		logger:           log.New(os.Stdout, "[adonis:http] ", log.LstdFlags),
	}
}

// SetRouter sets the router for the server.
func (s *Server) SetRouter(router contracts.RouterContract) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.router = router
}

// Use registers global middleware.
func (s *Server) Use(middleware ...contracts.MiddlewareFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.globalMiddleware = append(s.globalMiddleware, middleware...)
}

// RegisterNamed registers named middleware for route-level use.
func (s *Server) RegisterNamed(name string, middleware contracts.MiddlewareFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.namedMiddleware[name] = middleware
}

// ServeHTTP implements http.Handler — the core request handling pipeline.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create the HttpContext
	ctx := NewHttpContext(w, r)

	// Find the matching route
	route, params, found := s.router.FindRoute(r.Method, r.URL.Path)
	if !found {
		ctx.Response().Status(http.StatusNotFound).Json(map[string]any{ //nolint:errcheck
			"error":   "Not Found",
			"message": fmt.Sprintf("Cannot %s %s", r.Method, r.URL.Path),
			"status":  404,
		})
		return
	}

	// Set route params on context
	ctx.SetParams(params)

	// Build the middleware chain:
	// Global middleware -> Named middleware (from route) -> Handler
	s.mu.RLock()
	globalMw := make([]contracts.MiddlewareFunc, len(s.globalMiddleware))
	copy(globalMw, s.globalMiddleware)
	s.mu.RUnlock()

	// Collect named middleware for this route
	var routeMw []contracts.MiddlewareFunc
	for _, name := range route.GetMiddleware() {
		s.mu.RLock()
		if mw, ok := s.namedMiddleware[name]; ok {
			routeMw = append(routeMw, mw)
		}
		s.mu.RUnlock()
	}

	// Concatenate: global + route-specific
	allMiddleware := append(globalMw, routeMw...)

	// Build the chain and execute
	handler := route.Handler()
	chain := buildMiddlewareChain(ctx, allMiddleware, handler)

	if err := chain(); err != nil {
		// If the response hasn't been committed, send an error
		if !ctx.Response().IsCommitted() {
			ctx.Response().Status(http.StatusInternalServerError).Json(map[string]any{ //nolint:errcheck
				"error":   "Internal Server Error",
				"message": err.Error(),
				"status":  500,
			})
		}
		s.logger.Printf("❌ Error handling %s %s: %v", r.Method, r.URL.Path, err)
	}
}

// buildMiddlewareChain creates a nested function chain from middleware + final handler.
func buildMiddlewareChain(ctx *HttpContext, middleware []contracts.MiddlewareFunc, handler contracts.HandlerFunc) func() error {
	// Start from the innermost handler
	current := func() error {
		return handler(ctx)
	}

	// Wrap each middleware around it, from last to first
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		next := current
		current = func() error {
			return mw(ctx, next)
		}
	}

	return current
}

// Start begins listening for HTTP requests on the given address.
func (s *Server) Start(addr string) error {
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	s.mu.Unlock()

	s.logger.Printf("🚀 Server started on %s", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	srv := s.httpServer
	s.mu.RUnlock()

	if srv == nil {
		return nil
	}
	s.logger.Println("🔄 Shutting down HTTP server...")
	return srv.Shutdown(ctx)
}

// Ensure Server implements ServerContract at compile time.
var _ contracts.ServerContract = (*Server)(nil)
