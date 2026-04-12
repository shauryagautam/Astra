package http

import (
	"log/slog"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeMuxRouter(t *testing.T) {
	r := NewRouter(&config.AstraConfig{}, slog.Default())

	// 1. Static Routes
	r.Get("/hello", func(c *Context) error {
		return c.SendString("world")
	})

	// 2. Parametric Routes
	r.Get("/users/{id}", func(c *Context) error {
		return c.SendString("user:" + c.Param("id"))
	})

	r.Get("/users/{id}/posts/{post_id}", func(c *Context) error {
		return c.SendString(fmt.Sprintf("user:%s post:%s", c.Param("id"), c.Param("post_id")))
	})

	// 3. Conflict Resolution (Static should win over Param)
	r.Get("/users/me", func(c *Context) error {
		return c.SendString("me")
	})

	// 4. Wildcards
	r.Get("/static/*", func(c *Context) error {
		return c.SendString("static:" + c.Param("*"))
	})

	tests := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{"GET", "/hello", http.StatusOK, "world"},
		{"GET", "/users/123", http.StatusOK, "user:123"},
		{"GET", "/users/456/posts/789", http.StatusOK, "user:456 post:789"},
		{"GET", "/users/me", http.StatusOK, "me"},
		{"GET", "/static/css/style.css", http.StatusOK, "static:css/style.css"},
		{"GET", "/notfound", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
			if tt.body != "" {
				assert.Equal(t, tt.body, w.Body.String())
			}
		})
	}
}

func TestRouterGroups(t *testing.T) {
	r := NewRouter(&config.AstraConfig{}, slog.Default())
	
	r.Group("/api", func(api *Router) {
		api.Get("/v1", func(c *Context) error {
			return c.SendString("v1")
		})
	})

	req := httptest.NewRequest("GET", "/api/v1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v1", w.Body.String())
}

func TestRouterMiddleware(t *testing.T) {
	r := NewRouter(&config.AstraConfig{}, slog.Default())
	
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "true")
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/mid", func(c *Context) error {
		return c.NoContent()
	})

	req := httptest.NewRequest("GET", "/mid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "true", w.Header().Get("X-Middleware"))
}
