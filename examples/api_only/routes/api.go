package routes

import "github.com/shauryagautam/Astra/pkg/engine/http"

func Register(r *http.Router) {
	r.Get("/ping", func(c *http.Context) error {
		return c.JSON(map[string]string{"message": "pong"})
	})
}
