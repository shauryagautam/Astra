package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraframework/astra/core"
)

func BenchmarkRouter_Routing(b *testing.B) {
	app, _ := core.New()
	r := NewRouter(app)
	r.Get("/api/users/:id", func(c *Context) error {
		return c.SendString("OK")
	})

	handler := r.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/users/123", nil)
	rr := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkContext_JSONSerialize(b *testing.B) {
	app, _ := core.New()
	r := NewRouter(app)
	r.Get("/json", func(c *Context) error {
		return c.JSON(map[string]interface{}{
			"status":  "success",
			"message": "Hello, World!",
			"data": map[string]interface{}{
				"id":    1,
				"name":  "Test User",
				"roles": []string{"admin", "user"},
			},
		})
	})

	handler := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/json", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkContext_JSONDeserialize(b *testing.B) {
	app, _ := core.New()
	r := NewRouter(app)

	type Payload struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	r.Post("/json", func(c *Context) error {
		var p Payload
		if err := c.Bind(&p); err != nil {
			return err
		}
		return c.SendString("OK")
	})

	handler := r.Handler()

	body := []byte(`{"id": 1, "name": "Test Input"}`)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/json", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
