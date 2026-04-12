package test_util

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStorage is an in-memory file storage driver for test_util.
type MemoryStorage struct {
	files map[string][]byte
	mu    sync.RWMutex
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		files: make(map[string][]byte),
	}
}

// Put stores content in memory.
func (s *MemoryStorage) Put(ctx context.Context, path string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a copy to prevent mutation
	data := make([]byte, len(content))
	copy(data, content)

	s.files[path] = data
	return nil
}

// Get retrieves content from memory.
func (s *MemoryStorage) Get(ctx context.Context, path string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// Return a copy to prevent mutation
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// Delete removes content from memory.
func (s *MemoryStorage) Delete(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, path)
	return nil
}

// URL returns a fake URL for the file.
func (s *MemoryStorage) URL(path string) (string, error) {
	return "http://localhost/storage/" + path, nil
}

// SignedURL returns a fake signed URL for the file.
func (s *MemoryStorage) SignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	return "http://localhost/storage/" + path + "?signed=true", nil
}

// Exists checks if the file exists in memory.
func (s *MemoryStorage) Exists(ctx context.Context, path string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.files[path]
	return ok, nil
}

// Copy copies a file from source to destination.
func (s *MemoryStorage) Copy(ctx context.Context, src, dest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.files[src]
	if !ok {
		return fmt.Errorf("file not found: %s", src)
	}

	newData := make([]byte, len(data))
	copy(newData, data)
	s.files[dest] = newData
	return nil
}

// Move moves a file from source to destination.
func (s *MemoryStorage) Move(ctx context.Context, src, dest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.files[src]
	if !ok {
		return fmt.Errorf("file not found: %s", src)
	}

	s.files[dest] = data
	delete(s.files, src)
	return nil
}
