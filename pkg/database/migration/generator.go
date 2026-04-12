package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Generate creates a new migration file in the specified directory.
func Generate(dir, name string) (string, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("failed to create migrations directory: %w", err)
	}

	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", timestamp, name)
	path := filepath.Join(dir, filename)

	content := `-- +migrate Up
-- SQL in section 'Up' is executed when this migration is applied
	
-- +migrate Down
-- SQL section 'Down' is executed when this migration is rolled back
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to create migration file: %w", err)
	}

	return path, nil
}
