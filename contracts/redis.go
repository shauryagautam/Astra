package contracts

import (
	"context"
	"time"
)

// RedisContract defines the Redis module interface.
// Mirrors AdonisJS's @adonisjs/redis module.
type RedisContract interface {
	// Connection returns a named Redis connection.
	// Mirrors: Redis.connection('local')
	Connection(name string) RedisConnectionContract

	// Quit closes all Redis connections.
	Quit() error
}

// RedisConnectionContract defines operations on a single Redis connection.
type RedisConnectionContract interface {
	// --- String Commands ---
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)

	// --- Hash Commands ---
	HGet(ctx context.Context, key string, field string) (string, error)
	HSet(ctx context.Context, key string, field string, value any) error
	HDel(ctx context.Context, key string, fields ...string) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)

	// --- List Commands ---
	LPush(ctx context.Context, key string, values ...any) error
	RPush(ctx context.Context, key string, values ...any) error
	LPop(ctx context.Context, key string) (string, error)
	RPop(ctx context.Context, key string) (string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LRange(ctx context.Context, key string, start int64, stop int64) ([]string, error)
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error)

	// --- Set Commands ---
	SAdd(ctx context.Context, key string, members ...any) error
	SRem(ctx context.Context, key string, members ...any) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SIsMember(ctx context.Context, key string, member any) (bool, error)

	// --- Sorted Set Commands ---
	ZAdd(ctx context.Context, key string, score float64, member any) error
	ZRangeByScore(ctx context.Context, key string, min string, max string) ([]string, error)
	ZRem(ctx context.Context, key string, members ...any) error

	// --- Pub/Sub ---
	Publish(ctx context.Context, channel string, message any) error
	Subscribe(ctx context.Context, channels ...string) PubSubContract

	// --- Misc ---
	FlushDB(ctx context.Context) error
	Ping(ctx context.Context) error

	// --- Pipelining ---
	Pipeline() RedisPipeContract
}

// RedisPipeContract defines a pipeline for batching Redis commands.
type RedisPipeContract interface {
	LPush(ctx context.Context, key string, values ...any)
	ZRem(ctx context.Context, key string, members ...any)
	Exec(ctx context.Context) error
}

// PubSubContract defines the pub/sub subscriber interface.
type PubSubContract interface {
	// Channel returns a Go channel that receives messages.
	Channel() <-chan PubSubMessage

	// Close unsubscribes and closes the subscriber.
	Close() error
}

// PubSubMessage represents a message received via pub/sub.
type PubSubMessage struct {
	Channel string
	Payload string
}

// CacheContract defines the cache interface.
// Can be backed by Redis, memory, or other stores.
type CacheContract interface {
	// Get retrieves a cached value by key.
	Get(ctx context.Context, key string) (string, error)

	// Put stores a value with an optional TTL.
	Put(ctx context.Context, key string, value any, ttl time.Duration) error

	// Has checks if a key exists.
	Has(ctx context.Context, key string) (bool, error)

	// Forget removes a cached key.
	Forget(ctx context.Context, key string) error

	// Forever stores a value that never expires.
	Forever(ctx context.Context, key string, value any) error

	// Flush clears all cached data.
	Flush(ctx context.Context) error

	// Remember gets a cached value or executes the callback and caches the result.
	Remember(ctx context.Context, key string, ttl time.Duration, callback func() (any, error)) (string, error)

	// Increment increments a numeric value.
	Increment(ctx context.Context, key string, value ...int64) (int64, error)

	// Decrement decrements a numeric value.
	Decrement(ctx context.Context, key string, value ...int64) (int64, error)
}

// RateLimiterContract defines the rate limiter interface.
type RateLimiterContract interface {
	// Attempt checks if the key has exceeded the max attempts.
	// Returns true if the request is allowed.
	Attempt(ctx context.Context, key string, maxAttempts int, window time.Duration) (bool, error)

	// TooManyAttempts returns true if the key has exceeded max attempts.
	TooManyAttempts(ctx context.Context, key string, maxAttempts int) (bool, error)

	// RemainingAttempts returns the number of remaining attempts.
	RemainingAttempts(ctx context.Context, key string, maxAttempts int) (int, error)

	// Clear resets the counter for a key.
	Clear(ctx context.Context, key string) error
}
