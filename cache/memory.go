package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type memoryItem struct {
	value     string
	expiresAt time.Time
}

// MemoryStore implements Store with process-local memory.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

// NewMemoryStore creates a new in-memory cache store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]memoryItem),
	}
}

// Get retrieves a value from memory.
func (m *MemoryStore) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return "", ErrCacheMiss
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()
		return "", ErrCacheMiss
	}

	return item.value, nil
}

// Set stores a value in memory.
func (m *MemoryStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	m.items[key] = memoryItem{
		value:     fmt.Sprint(value),
		expiresAt: expiresAt,
	}
	return nil
}

// Delete removes a value from memory.
func (m *MemoryStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

// Has reports whether a value exists in memory.
func (m *MemoryStore) Has(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	_, err := m.Get(ctx, key)
	if errors.Is(err, ErrCacheMiss) {
		return false, nil
	}
	return err == nil, err
}

// Flush clears all in-memory values.
func (m *MemoryStore) Flush(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]memoryItem)
	return nil
}
