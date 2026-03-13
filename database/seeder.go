// Package database provides the seeder infrastructure for populating
// the database with test or production seed data.
package database

import (
	"context"
	"fmt"
	"sort"

	"github.com/astraframework/astra/orm"
)

var (
	// DefaultRunner is the global seeder runner used by the CLI.
	DefaultRunner = NewSeederRunner()
)

// Register adds one or more seeders to the default global runner.
func Register(seeders ...Seeder) {
	DefaultRunner.Register(seeders...)
}

// Seeder defines the interface that all seeders must implement.
type Seeder interface {
	// Name returns the unique name of this seeder (e.g. "01_users").
	Name() string
	// Run executes the seeder, inserting or upserting seed data.
	Run(ctx context.Context, db *orm.DB) error
}

// SeederRunner manages and executes registered seeders.
type SeederRunner struct {
	seeders []Seeder
	index   map[string]Seeder
}

// NewSeederRunner creates a new SeederRunner.
func NewSeederRunner() *SeederRunner {
	return &SeederRunner{
		index: make(map[string]Seeder),
	}
}

// Register adds one or more seeders to the runner in order.
func (r *SeederRunner) Register(seeders ...Seeder) {
	for _, s := range seeders {
		if _, exists := r.index[s.Name()]; !exists {
			r.seeders = append(r.seeders, s)
			r.index[s.Name()] = s
		}
	}
}

// Run executes all registered seeders in the order they were registered.
func (r *SeederRunner) Run(ctx context.Context, db *orm.DB) error {
	if len(r.seeders) == 0 {
		fmt.Println("  No seeders registered.")
		return nil
	}

	for _, s := range r.seeders {
		fmt.Printf("  Seeding: %s\n", s.Name())
		if err := s.Run(ctx, db); err != nil {
			return fmt.Errorf("seeder %q failed: %w", s.Name(), err)
		}
		fmt.Printf("  ✓ Done:   %s\n", s.Name())
	}
	return nil
}

// RunByName runs a specific seeder by its registered name.
func (r *SeederRunner) RunByName(ctx context.Context, db *orm.DB, name string) error {
	s, ok := r.index[name]
	if !ok {
		available := r.Names()
		return fmt.Errorf("seeder %q not found. Available: %v", name, available)
	}
	fmt.Printf("  Seeding: %s\n", s.Name())
	if err := s.Run(ctx, db); err != nil {
		return fmt.Errorf("seeder %q failed: %w", name, err)
	}
	fmt.Printf("  ✓ Done:   %s\n", s.Name())
	return nil
}

// Names returns all registered seeder names, sorted alphabetically.
func (r *SeederRunner) Names() []string {
	names := make([]string, 0, len(r.index))
	for k := range r.index {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
