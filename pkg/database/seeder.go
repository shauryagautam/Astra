// Package database provides explicit data-loading infrastructure for populating
// a database with test or production data.
package database

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// DataSet defines one explicit data-loading step. A DataSet has no package
// global registration; construct a Runner and register it at the composition
// root.
type DataSet interface {
	Name() string
	Run(ctx context.Context, db *DB) error
}

// Runner manages and executes explicitly registered data sets.
type Runner struct {
	mu      sync.RWMutex
	entries []DataSet
	index   map[string]DataSet
}

// NewRunner creates an explicit data runner.
func NewRunner() *Runner {
	return &Runner{
		index: make(map[string]DataSet),
	}
}

// Register adds one or more data sets to the runner in order.
func (r *Runner) Register(entries ...DataSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range entries {
		if s == nil {
			continue
		}
		if _, exists := r.index[s.Name()]; !exists {
			r.entries = append(r.entries, s)
			r.index[s.Name()] = s
		}
	}
}

// Run executes all registered data sets in registration order.
func (r *Runner) Run(ctx context.Context, db *DB) error {
	r.mu.RLock()
	entries := append([]DataSet(nil), r.entries...)
	r.mu.RUnlock()
	if len(entries) == 0 {
		fmt.Println("  No data sets registered.")
		return nil
	}

	for _, s := range entries {
		fmt.Printf("  Loading data: %s\n", s.Name())
		if err := s.Run(ctx, db); err != nil {
			return fmt.Errorf("data set %q failed: %w", s.Name(), err)
		}
		fmt.Printf("  ✓ Done:       %s\n", s.Name())
	}
	return nil
}

// RunByName runs a specific data set by name.
func (r *Runner) RunByName(ctx context.Context, db *DB, name string) error {
	r.mu.RLock()
	s, ok := r.index[name]
	r.mu.RUnlock()
	if !ok {
		available := r.Names()
		return fmt.Errorf("data set %q not found. Available: %v", name, available)
	}
	fmt.Printf("  Loading data: %s\n", s.Name())
	if err := s.Run(ctx, db); err != nil {
		return fmt.Errorf("data set %q failed: %w", name, err)
	}
	fmt.Printf("  ✓ Done:       %s\n", s.Name())
	return nil
}

// Names returns all registered data set names, sorted alphabetically.
func (r *Runner) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.index))
	for k := range r.index {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
