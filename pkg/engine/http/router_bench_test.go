package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

)

func BenchmarkRouter_FlatRoute(b *testing.B) {
	app := NewTestApp()
	r := NewRouter(app.Config(), app.Logger())
	r.Get("/ping", func(c *Context) error {
		return c.SendString("pong")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_ParamRoute(b *testing.B) {
	app := NewTestApp()
	r := NewRouter(app.Config(), app.Logger())
	r.Get("/user/{id}", func(c *Context) error {
		return c.SendString(c.Param("id"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/user/123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_MiddlewareStack(b *testing.B) {
	app := NewTestApp()
	r := NewRouter(app.Config(), app.Logger())

	// Add 5 empty middlewares
	for i := 0; i < 5; i++ {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		})
	}

	r.Get("/ping", func(c *Context) error {
		return c.SendString("pong")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}
