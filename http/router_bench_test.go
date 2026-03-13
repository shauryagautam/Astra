package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraframework/astra/core"
)

func BenchmarkRouter_FlatRoute(b *testing.B) {
	app, _ := core.New()
	r := NewRouter(app)
	r.Get("/ping", func(c *Context) error {
		return c.SendString("pong")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.mux.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_ParamRoute(b *testing.B) {
	app, _ := core.New()
	r := NewRouter(app)
	r.Get("/user/{id}", func(c *Context) error {
		return c.SendString(c.Param("id"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/user/123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.mux.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_MiddlewareStack(b *testing.B) {
	app, _ := core.New()
	r := NewRouter(app)

	// Add 5 empty middlewares
	for i := 0; i < 5; i++ {
		r.Use(func(next HandlerFunc) HandlerFunc {
			return func(c *Context) error {
				return next(c)
			}
		})
	}

	r.Get("/ping", func(c *Context) error {
		return c.SendString("pong")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.mux.ServeHTTP(w, req)
	}
}
