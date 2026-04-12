package handler

import (
	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/http"
)

type AuthController struct {
	app *engine.App
}

func NewAuthController(app *engine.App) *AuthController {
	return &AuthController{app: app}
}

func (c *AuthController) ShowLogin(ctx *http.Context) error {
	// If already logged in, redirect to dashboard
	if sess := ctx.Session(); sess != nil && sess.Get("user_id") != nil {
		return ctx.Redirect("/dashboard", 302)
	}
	return ctx.Render("login.html", map[string]any{
		"Title": "Login - Astra SSR",
	})
}

func (c *AuthController) Login(ctx *http.Context) error {
	email := ctx.Request.FormValue("email")
	password := ctx.Request.FormValue("password")

	// Dummy auth check
	if email == "admin@example.com" && password == "password" {
		sess := ctx.Session()
		if sess != nil {
			sess.Set("user_id", 1)
			sess.Set("user_email", email)
		}
		ctx.Flash("success", "Welcome back!")
		return ctx.Redirect("/dashboard", 302)
	}

	// Login failed
	ctx.Flash("error", "Invalid credentials")
	
	return ctx.Redirect("/login", 302)
}

func (c *AuthController) Logout(ctx *http.Context) error {
	if sess := ctx.Session(); sess != nil {
		sess.Delete("user_id")
		sess.Delete("user_email")
	}
	ctx.Flash("success", "You have been logged out.")
	return ctx.Redirect("/login", 302)
}

func (c *AuthController) Dashboard(ctx *http.Context) error {
	sess := ctx.Session()
	if sess == nil || sess.Get("user_id") == nil {
		ctx.Flash("error", "Please login to access the dashboard.")
		return ctx.Redirect("/login", 302)
	}

	return ctx.Render("dashboard.html", map[string]any{
		"Title": "Dashboard - Astra SSR",
		"Email": sess.Get("user_email"),
	})
}
