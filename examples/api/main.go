package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shaurya/adonis/app"
	exceptions "github.com/shaurya/adonis/app/Exceptions"
	adonisHttp "github.com/shaurya/adonis/app/Http"
	"github.com/shaurya/adonis/contracts"
	"github.com/shaurya/adonis/providers"
)

func main() {
	// ── Bootstrap ─────────────────────────────────────────────────────
	application := app.NewApplication(".")

	// Register core providers
	application.RegisterProviders([]contracts.ServiceProviderContract{
		providers.NewAppProvider(application),
		providers.NewRouteProvider(application),
		// Add other providers here (Auth, Database, Redis, etc.)
	})

	// Boot the application (loads providers, runs configs, etc.)
	if err := application.Boot(); err != nil {
		log.Fatalf("Failed to boot application: %v", err)
	}
	defer application.Shutdown() //nolint:errcheck

	// ── Global Exception Handler ──────────────────────────────────────
	server := application.Use("Server").(*adonisHttp.Server)

	// Check if APP_DEBUG is enabled
	env := application.Use("Env").(*providers.EnvManager)
	debug := env.GetBool("APP_DEBUG", true)

	server.SetExceptionHandler(exceptions.NewHandler(debug))

	// ── Routes ────────────────────────────────────────────────────────
	router := application.Use("Route").(*adonisHttp.Router)
	registerRoutes(router)

	// ── Server Setup ──────────────────────────────────────────────────
	port := env.Get("PORT", "3333")
	host := env.Get("HOST", "0.0.0.0")
	addr := fmt.Sprintf("%s:%s", host, port)

	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Printf("  🚀 AdonisGo Example API is running on http://%s\n", addr)
	fmt.Printf("  Environment: %s\n", env.Get("APP_ENV", "development"))
	fmt.Println("════════════════════════════════════════════════════════")

	// Start server in a goroutine
	go func() {
		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	fmt.Println("👋 Goodbye!")
}
