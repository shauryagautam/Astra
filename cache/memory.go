package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrCacheMiss = errors.New("cache: miss")

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

// MemoryStore implements the Store interface using an in-memory map.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

// NewMemoryStore creates a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]memoryItem),
	}
}

func (m *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	m.items[key] = memoryItem{
		value:     value,
		expiresAt: expiresAt,
	}
	return nil
}

func (m *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return nil, ErrCacheMiss
	}

	return item.value, nil
}

func (m *MemoryStore) Forget(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

func (m *MemoryStore) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]memoryItem)
	return nil
}
