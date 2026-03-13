package benchmarks

import (
	"net/http/httptest"
	"testing"

	"github.com/astraframework/astra/core"
	astrahttp "github.com/astraframework/astra/http"
)

func BenchmarkRouter_Routing_Static(b *testing.B) {
	app, _ := core.New()
	router := astrahttp.NewRouter(app)
	router.Get("/v1/users", func(c *astrahttp.Context) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/v1/users", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_Routing_Param(b *testing.B) {
	app, _ := core.New()
	router := astrahttp.NewRouter(app)
	router.Get("/v1/users/{id}", func(c *astrahttp.Context) error {
		return c.SendString(c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/v1/users/123", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}

func BenchmarkRouter_Middleware_Stack(b *testing.B) {
	app, _ := core.New()
	router := astrahttp.NewRouter(app)

	// Add 5 empty middleware
	for i := 0; i < 5; i++ {
		router.Use(func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
			return func(c *astrahttp.Context) error {
				return next(c)
			}
		})
	}

	router.Get("/v1/test", func(c *astrahttp.Context) error {
		return c.NoContent()
	})

	req := httptest.NewRequest("GET", "/v1/test", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.ServeHTTP(w, req)
	}
}
