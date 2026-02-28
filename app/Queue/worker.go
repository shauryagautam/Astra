package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Process starts the worker loop for a specific queue.
// It blocks until the context is cancelled.
func (q *RedisQueue) Process(queueName string) error {
	ctx := context.Background()
	queueKey := q.prefix + queueName
	q.logger.Printf("Worker started for queue: %s", queueName)

	// Start a goroutine to handle delayed jobs
	go q.pollDelayedJobs(ctx, queueName)

	for {
		// BRPop blocks until an element is available in the list
		// We use a 5-second timeout to allow for periodic health checks or loop exit
		items, err := q.redis.BRPop(ctx, 5*time.Second, queueKey)
		if err != nil {
			// Refined error handling: Check for redis.Nil which indicates a timeout in BRPop
			if err.Error() == "redis: nil" {
				continue // Normal timeout, no jobs available
			}

			q.logger.Printf("Redis connection error: %v", err)
			time.Sleep(2 * time.Second) // Backoff on actual connection errors
			continue
		}

		// items[0] is the key, items[1] is the value popped
		if len(items) < 2 {
			continue
		}

		var payload JobPayload
		if err := json.Unmarshal([]byte(items[1]), &payload); err != nil {
			q.logger.Printf("Failed to unmarshal job payload: %v", err)
			continue
		}

		// Execute job
		handler, ok := q.registry.Get(payload.Name)
		if !ok {
			q.logger.Printf("No handler registered for job: %s", payload.Name)
			continue
		}

		q.logger.Printf("Processing job: %s", payload.Name)
		start := time.Now()

		if err := handler(payload.Data); err != nil {
			q.logger.Printf("Job '%s' failed: %v", payload.Name, err)

			// Increment attempts
			payload.Attempts++

			if payload.Attempts < payload.MaxAttempts {
				delay := payload.Backoff
				if delay == 0 {
					delay = 5 // Default backoff if none specified
				}

				q.logger.Printf("Retrying job '%s' (Attempt %d/%d) in %d seconds",
					payload.Name, payload.Attempts+1, payload.MaxAttempts, delay)

				// Re-marshal and push to delayed queue
				payloadBytes, _ := json.Marshal(payload)
				executeAt := float64(time.Now().Add(time.Duration(delay) * time.Second).Unix())
				_ = q.redis.ZAdd(ctx, q.prefix+"delayed", executeAt, payloadBytes)
			} else {
				q.logger.Printf("Job '%s' failed after %d attempts. Moving to failed queue.",
					payload.Name, payload.MaxAttempts)

				// Push to failed jobs list for manual inspection/retry
				payloadBytes, _ := json.Marshal(payload)
				failedKey := q.prefix + "failed"
				if pushErr := q.redis.LPush(ctx, failedKey, payloadBytes); pushErr != nil {
					q.logger.Printf("CRITICAL: Failed to push job '%s' to failed queue: %v", payload.Name, pushErr)
				}
			}
		} else {
			q.logger.Printf("Job '%s' finished in %s", payload.Name, time.Since(start))
		}
	}
}

// pollDelayedJobs moves jobs from the delayed set to the main queue when they are due.
func (q *RedisQueue) pollDelayedJobs(ctx context.Context, targetQueue string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	delayedKey := q.prefix + "delayed"
	targetKey := q.prefix + targetQueue

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Fetch jobs where score <= now
			now := time.Now().Unix()
			jobs, err := q.redis.ZRangeByScore(ctx, delayedKey, "-inf", fmt.Sprintf("%d", now))

			if err != nil || len(jobs) == 0 {
				continue
			}

			if len(jobs) > 0 {
				pipe := q.redis.Pipeline()
				for _, jobBody := range jobs {
					pipe.LPush(ctx, targetKey, jobBody)
					pipe.ZRem(ctx, delayedKey, jobBody)
				}
				if err := pipe.Exec(ctx); err != nil {
					q.logger.Printf("Pipeline execution failed in delayed poller: %v", err)
				}
			}
		}
	}
}
