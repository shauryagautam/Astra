package queue

import "github.com/redis/go-redis/v9"

// StreamsDriver is the compatibility alias for the Redis-backed dispatcher.
type StreamsDriver = RedisDispatcher

// NewStreamsDriver creates a Redis Streams dispatcher.
func NewStreamsDriver(client redis.UniversalClient, prefix string) *StreamsDriver {
	return NewRedisDispatcher(client, prefix)
}
