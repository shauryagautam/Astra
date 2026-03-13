package routes

import (
	"github.com/astraframework/astra/http"
	"api_only/controllers"
)

func Register(r *http.Router) {
	r.Get("/ping", func(c *http.Context) error {
		return c.JSON(map[string]string{"message": "pong"})
	})

	todoCtrl := controllers.NewTodoController(r.App)
	
	r.Get("/todos", todoCtrl.List)
	r.Post("/todos", todoCtrl.Create)
}
