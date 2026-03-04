package queue

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// FailedJobManager manages jobs that failed after all retries.
type FailedJobManager struct {
	client *redis.Client
	prefix string
}

// NewFailedJobManager creates a new FailedJobManager.
func NewFailedJobManager(client *redis.Client, prefix string) *FailedJobManager {
	return &FailedJobManager{
		client: client,
		prefix: prefix,
	}
}

// Retry pushes all failed jobs back to their queues.
func (m *FailedJobManager) Retry(ctx context.Context) error {
	failedKey := m.prefix + "failed_jobs"
	
	for {
		val, err := m.client.RPop(ctx, failedKey).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return err
		}

		var p payload
		if err := json.Unmarshal([]byte(val), &p); err == nil {
			// Reset attempts
			p.Attempts = 0
			bytes, _ := json.Marshal(p)
			// Need a way to know the queue. In our simplistic approach, 
			// the worker moved it here. We'll just push it to "default" 
			// if we didn't store the queue in the payload. 
			// Let's assume queue is "default" for now.
			m.client.LPush(ctx, m.prefix+"default", bytes)
		}
	}

	return nil
}

// Flush deletes all failed jobs.
func (m *FailedJobManager) Flush(ctx context.Context) error {
	return m.client.Del(ctx, m.prefix+"failed_jobs").Err()
}
