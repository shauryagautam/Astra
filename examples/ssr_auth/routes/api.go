package routes

import (
	"github.com/astraframework/astra/http"
	"ssr_auth/controllers"
)

func Register(r *http.Router) {
	// Let's redirect / to /dashboard
	r.Get("/", func(c *http.Context) error {
		return c.Redirect("/dashboard", 302)
	})

	authCtrl := controllers.NewAuthController(r.App)

	r.Get("/login", authCtrl.ShowLogin)
	r.Post("/login", authCtrl.Login)
	r.Post("/logout", authCtrl.Logout)
	
	// Protected route
	r.Get("/dashboard", authCtrl.Dashboard)
}
