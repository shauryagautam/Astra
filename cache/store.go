package cache

import (
	"context"
	"errors"
	"time"
)

// ErrCacheMiss is returned when a cache key does not exist.
var ErrCacheMiss = errors.New("astra/cache: key not found")

// Store defines the cache contract used throughout Astra.
type Store interface {
	// Get retrieves a cached value.
	Get(ctx context.Context, key string) (string, error)
	// Set stores a value for the provided TTL. A zero TTL stores the value forever.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	// Delete removes a value from the cache.
	Delete(ctx context.Context, key string) error
	// Has reports whether a value exists in the cache.
	Has(ctx context.Context, key string) (bool, error)
	// Flush removes every key owned by the store.
	Flush(ctx context.Context) error
}
