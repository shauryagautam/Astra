package main

import (
	"context"
	"fmt"
	"log"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/http"
	"github.com/astraframework/astra/orm"
	"github.com/astraframework/astra/storage"
	"github.com/astraframework/astra/validate"

	"api_only/routes"
)

func main() {
	app, err := core.New()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	// Register ORM provider
	app.Use(&orm.ORMProvider{})

	// Register storage provider
	app.Use(storage.NewProvider())

	// Register a simple validation provider manually for the example
	app.Use(&validateProvider{})

	// Initialize the Database schema for the example
	app.OnStart(func(ctx context.Context) error {
		dbRaw := app.Get("db")
		if dbRaw == nil {
			return fmt.Errorf("database not initialized")
		}
		db := dbRaw.(*orm.DB)
		
		// Create the table (in a real app, use migrations)
		_, err := db.Exec(context.Background(), `
			CREATE TABLE IF NOT EXISTS todos (
				id SERIAL PRIMARY KEY,
				title VARCHAR(255) NOT NULL,
				completed BOOLEAN DEFAULT FALSE,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
		
		fmt.Println("Todos table ready!")
		return nil
	})

	// Register HTTP Router
	router := http.NewRouter(app)
	routes.Register(router)
	
	app.Use(http.NewHTTPProvider(router.Handler()))

	fmt.Println("Starting API Server on :3333")
	if err := app.Start(); err != nil {
		log.Fatalf("App failed: %v", err)
	}
}

// Simple Validate Provider to wire validate.Validator into container
type validateProvider struct {
	core.BaseProvider
}

func (p *validateProvider) Register(app *core.App) error {
	var opts []validate.ValidatorOption
	
	// If DB is available, wire it in for exists/unique rules
	if dbRaw := app.Get("db"); dbRaw != nil {
		if db, ok := dbRaw.(validate.DBExecutor); ok {
			opts = append(opts, validate.WithDB(db))
		}
	}
	
	app.Register("validator", validate.New(opts...))
	return nil
}
