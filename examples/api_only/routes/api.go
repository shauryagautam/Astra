package routes

import (
	"api_only/handler"
	"github.com/shauryagautam/Astra/pkg/engine/http"
)

func Register(r *http.Router) {
	// Standard library net/http style within the Astra router
	r.Get("/ping", func(c *http.Context) error {
		return c.JSON(map[string]string{"message": "pong"})
	})

	// Explicitly wired handlers with struct-based DI
	listHandler := &handler.ListTodoHandler{
		DB: r.App.DB(),
	}
	createHandler := &handler.CreateTodoHandler{
		DB:        r.App.DB(),
		Validator: r.App.Validator(),
	}

	// Future-proofing: Using Router.Handle which currently takes HandlerFunc
	// But in Phase 2 we will standardize this to http.Handler.
	// For now, we adapt our ServeHTTP to Astra's HandlerFunc if needed,
	// or use a helper that wraps http.Handler.
	
	r.Get("/todos", adapt(listHandler))
	r.Post("/todos", adapt(createHandler))
}

// adapt is a temporary shim until Phase 2 standardized the router to http.Handler.
func adapt(h http.Handler) http.HandlerFunc {
	return func(c *http.Context) error {
		h.ServeHTTP(c.Writer, c.Request)
		return nil
	}
}
