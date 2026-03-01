package database

import (
	"fmt"
	"log"
	"os"

	"gorm.io/gorm"
)

// Seeder represents a single database seeder.
// Mirrors Astra's seeder files.
type Seeder struct {
	// Name is the seeder name.
	Name string

	// Run executes the seeder.
	Run func(db *gorm.DB) error
}

// SeederRunner manages running database seeders.
type SeederRunner struct {
	db      *gorm.DB
	seeders []Seeder
	logger  *log.Logger
}

// NewSeederRunner creates a new seeder runner.
func NewSeederRunner(db *gorm.DB) *SeederRunner {
	return &SeederRunner{
		db:      db,
		seeders: make([]Seeder, 0),
		logger:  log.New(os.Stdout, "[astra:seeder] ", log.LstdFlags),
	}
}

// Add registers seeders.
func (r *SeederRunner) Add(seeders ...Seeder) {
	r.seeders = append(r.seeders, seeders...)
}

// Run executes all registered seeders.
// Mirrors: node ace db:seed
func (r *SeederRunner) Run() error {
	if len(r.seeders) == 0 {
		r.logger.Println("No seeders registered")
		return nil
	}

	r.logger.Printf("🌱 Running %d seeder(s)...", len(r.seeders))

	for _, seeder := range r.seeders {
		r.logger.Printf("▶ Seeding: %s", seeder.Name)
		if err := seeder.Run(r.db); err != nil {
			return fmt.Errorf("seeder '%s' failed: %w", seeder.Name, err)
		}
		r.logger.Printf("✅ Completed: %s", seeder.Name)
	}

	r.logger.Println("✅ All seeders completed")
	return nil
}

// RunByName runs a specific seeder by name.
func (r *SeederRunner) RunByName(name string) error {
	for _, seeder := range r.seeders {
		if seeder.Name == name {
			r.logger.Printf("🌱 Running seeder: %s", name)
			if err := seeder.Run(r.db); err != nil {
				return fmt.Errorf("seeder '%s' failed: %w", name, err)
			}
			r.logger.Printf("✅ Completed: %s", name)
			return nil
		}
	}
	return fmt.Errorf("seeder '%s' not found", name)
}
