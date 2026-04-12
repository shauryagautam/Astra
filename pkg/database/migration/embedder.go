package migration

import (
	"embed"
	"io/fs"
)

// Embedder allows embedding migration SQL files directly into the Go binary.
type Embedder struct {
	FS embed.FS
}

// ReadDir returns the migration files.
func (e *Embedder) ReadDir(name string) ([]fs.DirEntry, error) {
	return e.FS.ReadDir(name)
}

// ReadFile reads a specific migration file.
func (e *Embedder) ReadFile(name string) ([]byte, error) {
	return e.FS.ReadFile(name)
}
