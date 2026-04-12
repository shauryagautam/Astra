package benchmark

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
)

func BenchmarkRouter_Routing_Static(b *testing.B) {
	cfg := &config.AstraConfig{}
	app := engine.New(
		cfg,
		&config.Config{},
		slog.Default(),
	)
	router := astrahttp.NewRouter(cfg, app.Logger())
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
	cfg := &config.AstraConfig{}
	app := engine.New(
		cfg,
		&config.Config{},
		slog.Default(),
	)
	router := astrahttp.NewRouter(cfg, app.Logger())
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
	cfg := &config.AstraConfig{}
	app := engine.New(
		cfg,
		&config.Config{},
		slog.Default(),
	)
	router := astrahttp.NewRouter(cfg, app.Logger())

	// Add 5 empty middleware
	for i := 0; i < 5; i++ {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
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
