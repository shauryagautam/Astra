package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/engine"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
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

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// 3. Initialize Decoupled Router
	router := astrahttp.NewRouter(cfg, logger)
	
	log.Printf("Starting Astra server on %s", addr)
	
	// Start server (simplified bootstrap)
	go func() {
		if err := http.ListenAndServe(addr, router); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatalf("app failed: %v", err)
	}
}
