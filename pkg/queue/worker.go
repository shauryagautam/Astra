package queue

import (
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// Worker is the compatibility alias for the Redis-backed worker.
type Worker = RedisWorker

// NewWorker creates a Redis-backed worker.
func NewWorker(client redis.UniversalClient, prefix string, queues []string, concurrency int, logger *slog.Logger) *Worker {
	return NewRedisWorker(client, prefix, queues, logger).WithConcurrency(concurrency)
}
