package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStorage implements the Storage interface in-memory for test_util.
type MemoryStorage struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		files: make(map[string][]byte),
	}
}

func (s *MemoryStorage) Put(ctx context.Context, path string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = content
	return nil
}

func (s *MemoryStorage) Get(ctx context.Context, path string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return content, nil
}

func (s *MemoryStorage) Delete(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, path)
	return nil
}

func (s *MemoryStorage) URL(path string) (string, error) {
	return "memory://" + path, nil
}

func (s *MemoryStorage) SignedURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	return s.URL(path)
}

func (s *MemoryStorage) Exists(ctx context.Context, path string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.files[path]
	return ok, nil
}

func (s *MemoryStorage) Copy(ctx context.Context, src, dest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, ok := s.files[src]
	if !ok {
		return fmt.Errorf("source file not found: %s", src)
	}
	s.files[dest] = content
	return nil
}

func (s *MemoryStorage) Move(ctx context.Context, src, dest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, ok := s.files[src]
	if !ok {
		return fmt.Errorf("source file not found: %s", src)
	}
	s.files[dest] = content
	delete(s.files, src)
	return nil
}
