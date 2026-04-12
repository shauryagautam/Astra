package cache

import (
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Standalone constructors allow using the Astra cache package in any Go project
// without pulling in the full framework framework. Add to your go.mod with:
//
//	go get github.com/shauryagautam/Astra/pkg/cache
//
// Then use directly:
//
//	store, err := cache.NewRedisStandalone("localhost:6379", "", 0)
//	store, err := cache.NewRedisStandaloneURL("redis://localhost:6379")
//	store  = cache.NewMemoryStore()            // no deps at all

// NewRedisStandalone creates a Redis-backed cache Store using only a host address,
// password, and DB index. Requires no engine.App or extra framework setup.
//
//	store, err := cache.NewRedisStandalone("localhost:6379", "", 0)
func NewRedisStandalone(addr, password string, db int) (Store, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return NewRedisStore(client, ""), nil
}

// NewRedisStandaloneURL creates a Redis-backed cache Store from a Redis URL.
// Supports redis://, rediss://, and redis+sentinel:// schemes.
//
//	store, err := cache.NewRedisStandaloneURL("redis://:password@localhost:6379/0")
func NewRedisStandaloneURL(rawURL string) (Store, error) {
	opts, err := goredis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("cache.NewRedisStandaloneURL: %w", err)
	}
	client := goredis.NewClient(opts)
	return NewRedisStore(client, ""), nil
}

// NewMemoryStandalone creates an in-memory cache Store with zero external
// dependencies. Suitable for testing, CLI tools, and services that don't need
// distributed caching.
//
// Note: data is not persisted across process restarts.
func NewMemoryStandalone() Store {
	return NewMemoryStore()
}
