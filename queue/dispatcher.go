package queue

import "github.com/redis/go-redis/v9"

// Dispatcher is the compatibility alias for the Redis-backed dispatcher.
type Dispatcher = RedisDispatcher

// NewDispatcher creates a Redis-backed job dispatcher.
func NewDispatcher(client redis.UniversalClient, prefix string) *Dispatcher {
	return NewRedisDispatcher(client, prefix)
}
