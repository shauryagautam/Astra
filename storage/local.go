package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage implements the Storage interface for the local filesystem.
type LocalStorage struct {
	rootDir string
}

// NewLocalStorage creates a new LocalStorage.
func NewLocalStorage(rootDir string) *LocalStorage {
	return &LocalStorage{rootDir: rootDir}
}

// Put writes a file to the local filesystem.
func (s *LocalStorage) Put(ctx context.Context, path string, content []byte) error {
	fullPath := filepath.Join(s.rootDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(fullPath, content, 0644)
}

// Get reads a file from the local filesystem.
func (s *LocalStorage) Get(ctx context.Context, path string) ([]byte, error) {
	fullPath := filepath.Join(s.rootDir, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return io.ReadAll(file)
}

// Delete removes a file from the local filesystem.
func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(s.rootDir, path)
	return os.Remove(fullPath)
}

// URL returns a relative URL for the file.
func (s *LocalStorage) URL(path string) (string, error) {
	return "/storage/" + path, nil
}

// SignedURL returns a relative URL for the file (local storage doesn't presign).
func (s *LocalStorage) SignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	return s.URL(path)
}

// Exists checks if a file exists on the local filesystem.
func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(s.rootDir, path)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check file existence: %w", err)
}
