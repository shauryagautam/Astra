package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"

	"github.com/shauryagautam/Astra/pkg/engine/config"
)

// Router represents the Astra HTTP router.
// It is fully decoupled from the engine.App kernel and accepts explicit dependencies.
type Router struct {
	mux            *http.ServeMux
	Config         *config.AstraConfig
	Logger         *slog.Logger
	middleware     []MiddlewareFunc
	prefix         string
	trustedProxies []netip.Prefix // parsed once at construction from config
}

// NewRouter creates a new Astra HTTP router. It parses the TRUSTED_PROXIES
// configuration once at startup so per-request IP resolution is allocation-free.
func NewRouter(cfg *config.AstraConfig, logger *slog.Logger) *Router {
	var proxies []netip.Prefix
	if cfg != nil {
		proxies = ParseTrustedProxies(cfg.App.TrustedProxies)
	}
	return &Router{
		mux:            http.NewServeMux(),
		Config:         cfg,
		Logger:         logger,
		middleware:     make([]MiddlewareFunc, 0),
		trustedProxies: proxies,
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := NewContext(w, req)
	defer c.release()

	// Inject parsed trusted proxies so ClientIP() resolves the real client
	// address correctly when the server sits behind a reverse proxy.
	c.TrustedProxies = r.trustedProxies

	// Inject into request context
	ctx := context.WithValue(req.Context(), astraContextKey, c)
	
	// Delegate to the multiplexer with the injected context
	r.mux.ServeHTTP(w, req.WithContext(ctx))
}

func (r *Router) Get(path string, h HandlerFunc) {
	r.HandleContext(http.MethodGet, path, h)
}

func (r *Router) Post(path string, h HandlerFunc) {
	r.HandleContext(http.MethodPost, path, h)
}

func (r *Router) Put(path string, h HandlerFunc) {
	r.HandleContext(http.MethodPut, path, h)
}

func (r *Router) Delete(path string, h HandlerFunc) {
	r.HandleContext(http.MethodDelete, path, h)
}

func (r *Router) Patch(path string, h HandlerFunc) {
	r.HandleContext(http.MethodPatch, path, h)
}

// Handle registers a standard http.Handler.
func (r *Router) Handle(method, path string, h http.Handler) {
	fullPath := r.prefix + path
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}
	pattern := method + " " + fullPath
	
	r.mux.Handle(pattern, h)
}

// HandleContext registers an Astra-style HandlerFunc.
func (r *Router) HandleContext(method, path string, h HandlerFunc) {
	fullPath := r.prefix + path
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}

	// Convert Astra path syntax to Go 1.22+ ServeMux syntax
	muxPath := fullPath
	if strings.HasSuffix(muxPath, "/*") {
		muxPath = strings.TrimSuffix(muxPath, "/*") + "/{_wildcard...}"
	} else if strings.Contains(muxPath, "/*/") {
		muxPath = strings.ReplaceAll(muxPath, "/*/", "/{_wildcard...}/")
	}

	pattern := method + " " + muxPath

	// 1. Wrap the Astra HandlerFunc into a standard http.Handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c := FromRequest(req)
		if c == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":{"code":"INTERNAL_ERROR","message":"request context unavailable"}}`)
			return
		}

		c.Request = req

		if err := h(c); err != nil {
			logger := r.Logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Error("handler error", "error", err, "path", req.URL.Path)
			if !c.written {
				// Return a structured JSON error envelope so API clients never
				// receive a bare plain-text string on unhandled handler errors.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"error":{"code":"INTERNAL_ERROR","message":"an unexpected error occurred"}}`)
			}
		}
	})

	// 2. Wrap with the middleware chain (right-to-left)
	var final http.Handler = finalHandler
	for i := len(r.middleware) - 1; i >= 0; i-- {
		final = r.middleware[i](final)
	}

	// 3. Register on the mux
	r.mux.Handle(pattern, final)
}

func (r *Router) Group(prefix string, fn func(*Router)) {
	sub := &Router{
		mux:            r.mux,
		Config:         r.Config,
		Logger:         r.Logger,
		middleware:     append([]MiddlewareFunc{}, r.middleware...),
		prefix:         r.prefix + prefix,
		trustedProxies: r.trustedProxies,
	}
	fn(sub)
}

func (r *Router) Use(m MiddlewareFunc) {
	r.middleware = append(r.middleware, m)
}

// ProvideRouter provides the Astra Router for Wire dependency injection.
func ProvideRouter(cfg *config.AstraConfig, logger *slog.Logger) *Router {
	return NewRouter(cfg, logger)
}
