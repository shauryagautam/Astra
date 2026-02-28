package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/shaurya/adonis/app"
	"github.com/shaurya/adonis/contracts"
	"github.com/shaurya/adonis/providers"
)

func main() {
	// ── Bootstrap ─────────────────────────────────────────────────────
	application := app.NewApplication(".")

	// Register ALL providers (including new ones)
	application.RegisterProviders([]contracts.ServiceProviderContract{
		providers.NewAppProvider(application),
		providers.NewRouteProvider(application),
		providers.NewRedisProvider(application),
		providers.NewEventProvider(application),
		providers.NewQueueProvider(application),
		providers.NewDriveProvider(application),
		providers.NewMailProvider(application),
		providers.NewWsProvider(application),
	})

	// Boot the application
	if err := application.Boot(); err != nil {
		log.Fatalf("Failed to boot application: %v", err)
	}
	defer application.Shutdown()

	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Printf("  ⚡ AdonisGo Advanced Example running\n")
	fmt.Println("════════════════════════════════════════════════════════")

	// ── Setup Advanced Features ───────────────────────────────────────
	registerAdvancedRoutes(application)
	registerAdvancedEvents(application)
	registerAdvancedJobs(application)

	// ── Start Server ──────────────────────────────────────────────────
	server := application.Use("Server").(interface{ Start(string) error })
	env := application.Use("Env").(interface{ Get(string, string) string })

	addr := fmt.Sprintf(":%s", env.Get("PORT", "3333"))

	go func() {
		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 Graceful shutdown...")
	// Timeout for shutdown
}
