package main

import (
	"log"
	nethttp "net/http"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/http"
	"github.com/shauryagautam/Astra/pkg/session"
	"github.com/shauryagautam/Astra/pkg/storage"
	
	"ssr_auth/routes"
)

func main() {
	app, err := framework.New()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	// Register session provider for auth state and flash messages
	app.Use(session.NewProvider(session.Config{
		Store:      session.NewCookieStore([]byte("supersecret32bytesminimum1234567")),
		CookieName: "astra_session",
		MaxAge:     86400 * 30, // 30 days
		Secure:     false,      // For local dev
		SameSite:   nethttp.SameSiteLaxMode,
	}))

	// Register storage provider
	app.Use(storage.NewProvider())

	// Configure Template Engine for SSR Views
	// This makes it available as `views` to `c.Render`
	engine := http.NewTemplateEngine("views")
	app.SetViews(engine)

	// Register HTTP Router
	router := http.NewRouter(app)
	routes.Register(router)
	
	// Ensure the session middleware wraps all HTTP routes 
	// (in a real app you might only wrap specific route groups)
	// Use explicit SessionStore access
	if sessProvider, ok := app.SessionStore().(*session.Provider); ok {
		router.UseStd(sessProvider.Middleware())
	}

	app.Use(http.NewHTTPProvider(router.Handler()))

	log.Println("Starting SSR Auth Server on :3333")
	if err := app.Start(); err != nil {
		log.Fatalf("App failed: %v", err)
	}
}
