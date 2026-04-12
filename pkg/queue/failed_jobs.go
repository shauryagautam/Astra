package queue

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// FailedJobManager provides the legacy failed job API on top of the Redis store.
type FailedJobManager struct {
	store *RedisFailedJobsStore
}

// NewFailedJobManager creates a new failed job manager.
func NewFailedJobManager(client redis.UniversalClient, prefix string) *FailedJobManager {
	return &FailedJobManager{
		store: NewRedisFailedJobsStore(client, prefix, NewRedisQueue(client, prefix, nil)),
	}
}

// Retry re-enqueues all failed jobs.
func (m *FailedJobManager) Retry(ctx context.Context) error {
	jobs, err := m.store.All(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := m.store.Retry(ctx, job.ID); err != nil {
			return err
		}
	}
	return nil
}

// Flush deletes all failed jobs.
func (m *FailedJobManager) Flush(ctx context.Context) error {
	return m.store.Purge(ctx)
}
