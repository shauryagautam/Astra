package main

import (
	"log"
	"log/slog"
	nethttp "net/http"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/engine/http"
	"github.com/shauryagautam/Astra/pkg/session"

	"ssr_auth/routes"
)

func main() {
	// 1. Load configuration
	rawConfig, err := config.Load(".")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	cfg := config.LoadFromEnv(rawConfig)
	logger := slog.Default()

	// 2. Initialize App Lifecycle Manager
	app := engine.New(cfg, rawConfig, logger)

	// Create cookie store for sessions
	store := session.NewCookieStore([]byte("supersecret32bytesminimum1234567"))

	// Configure Template Engine for SSR Views
	viewEngine := http.NewTemplateEngine("views")

	// Register HTTP Router
	router := http.NewRouter(cfg, logger)

	// Register session middleware on the router
	router.Use(http.SessionMiddleware(store))

	// Register view engine middleware to set ViewEngine on context
	router.Use(func(next nethttp.Handler) nethttp.Handler {
		return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			if c := http.FromRequest(r); c != nil {
				c.ViewEngine = viewEngine
			}
			next.ServeHTTP(w, r)
		})
	})

	routes.Register(router, app)

	if err := app.RegisterComponent(http.NewServer(":3333", router, cfg.App.MaxBodySize)); err != nil {
		log.Fatalf("register server: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("App failed: %v", err)
	}
}
