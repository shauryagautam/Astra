// File: server.go — The main entry point for the Adonis Go framework.
// This is the equivalent of AdonisJS's server.ts file.
//
// It bootstraps the Application, registers Service Providers,
// triggers the lifecycle (register → boot → ready), registers routes
// and middleware, and starts the HTTP server.
//
// Usage:
//
//	go run server.go
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
	adonisHttp "github.com/shaurya/adonis/app/Http"
	"github.com/shaurya/adonis/config"
	"github.com/shaurya/adonis/contracts"
	"github.com/shaurya/adonis/providers"
	"github.com/shaurya/adonis/start"
)

func main() {
	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  1. Create the Application (The IoC Container)                   ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	application := app.NewApplication(".")
	application.SetAppName("AdonisGo")

	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  2. Load Configuration                                           ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	appConfig := config.DefaultAppConfig()
	appConfig.Name = "AdonisGo"

	corsConfig := config.DefaultCorsConfig()

	// Override from environment variables if present
	if port := os.Getenv("PORT"); port != "" {
		fmt.Sscanf(port, "%d", &appConfig.Port)
	}
	if host := os.Getenv("HOST"); host != "" {
		appConfig.Host = host
	}

	// Register config in the container
	application.Singleton("Adonis/Core/Config", func(c contracts.ContainerContract) (any, error) {
		return &appConfig, nil
	})
	application.Alias("Config", "Adonis/Core/Config")

	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  3. Register Service Providers                                   ║
	// ║  Mirrors AdonisJS's .adonisrc.json providers array               ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	application.RegisterProviders([]contracts.ServiceProviderContract{
		providers.NewAppProvider(application),
		providers.NewRouteProvider(application),
	})

	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  4. Boot the Application                                         ║
	// ║  Executes: Register() → Boot() on all providers                  ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	if err := application.Boot(); err != nil {
		log.Fatalf("❌ Failed to boot application: %v", err)
	}

	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  5. Register Routes & Middleware                                  ║
	// ║  Mirrors AdonisJS's start/routes.ts and start/kernel.ts          ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	start.RegisterMiddleware(application, corsConfig)
	start.RegisterRoutes(application)

	// Commit routes (finalizes the route tree)
	router := application.Use("Route").(*adonisHttp.Router)
	router.Commit()

	// Print registered routes
	fmt.Println("\n📋 Registered Routes:")
	fmt.Println(router.PrintRoutes())

	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  6. Signal the Application is Ready                              ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	if err := application.Ready(); err != nil {
		log.Fatalf("❌ Failed to signal ready: %v", err)
	}

	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  7. Start the HTTP Server                                        ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	server := application.Use("Server").(*adonisHttp.Server)
	addr := fmt.Sprintf("%s:%d", appConfig.Host, appConfig.Port)

	// Print the startup banner
	printBanner(appConfig)

	// Start server in a goroutine
	go func() {
		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	// ╔═══════════════════════════════════════════════════════════════════╗
	// ║  8. Graceful Shutdown                                            ║
	// ║  Listen for SIGINT/SIGTERM and drain connections                  ║
	// ╚═══════════════════════════════════════════════════════════════════╝
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n⏳ Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️  HTTP server shutdown error: %v", err)
	}
	if err := application.Shutdown(); err != nil {
		log.Printf("⚠️  Application shutdown error: %v", err)
	}

	fmt.Println("👋 Goodbye!")
}

// printBanner prints the Adonis Go startup banner.
func printBanner(cfg config.AppConfig) {
	banner := `
╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║     █████╗ ██████╗  ██████╗ ███╗   ██╗██╗███████╗        ║
║    ██╔══██╗██╔══██╗██╔═══██╗████╗  ██║██║██╔════╝        ║
║    ███████║██║  ██║██║   ██║██╔██╗ ██║██║███████╗        ║
║    ██╔══██║██║  ██║██║   ██║██║╚██╗██║██║╚════██║        ║
║    ██║  ██║██████╔╝╚██████╔╝██║ ╚████║██║███████║        ║
║    ╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚═╝╚══════╝        ║
║                                                           ║
║                  ⚡ Powered by Go ⚡                       ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝`
	fmt.Println(banner)
	fmt.Printf("\n  🚀 %s server started\n", cfg.Name)
	fmt.Printf("  📡 Listening on http://%s:%d\n", cfg.Host, cfg.Port)
	fmt.Printf("  🌍 Environment: %s\n", cfg.Environment)
	fmt.Println()
}
