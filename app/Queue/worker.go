package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Process starts the worker loop for a specific queue.
// It blocks until the context is cancelled.
func (q *RedisQueue) Process(queueName string) error {
	ctx := context.Background()
	queueKey := q.prefix + queueName
	logger := log.New(os.Stdout, "[adonis:queue] ", log.LstdFlags)

	logger.Printf("Worker started for queue: %s", queueName)

	// Start a goroutine to handle delayed jobs
	go q.pollDelayedJobs(ctx, queueName)

	for {
		// BRPop blocks until an element is available in the list
		// We use a 5-second timeout to allow for periodic health checks or loop exit
		items, err := q.redis.BRPop(ctx, 5*time.Second, queueKey)
		if err != nil {
			// In our contract, we'll assume a generic error for now
			// but could refine this based on the actual driver response.
			if err.Error() == "redis: nil" {
				continue // Timeout, no jobs
			}
			logger.Printf("Redis error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// items[0] is the key, items[1] is the value popped
		if len(items) < 2 {
			continue
		}

		var payload JobPayload
		if err := json.Unmarshal([]byte(items[1]), &payload); err != nil {
			logger.Printf("Failed to unmarshal job payload: %v", err)
			continue
		}

		// Execute job
		handler, ok := q.registry.Get(payload.Name)
		if !ok {
			logger.Printf("No handler registered for job: %s", payload.Name)
			continue
		}

		logger.Printf("Processing job: %s", payload.Name)
		start := time.Now()

		if err := handler(payload.Data); err != nil {
			logger.Printf("Job '%s' failed: %v", payload.Name, err)
			// Simple retry logic could be added here
		} else {
			logger.Printf("Job '%s' finished in %s", payload.Name, time.Since(start))
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

			for _, jobBody := range jobs {
				// Move to main queue
				// We don't have pipeline in our contract yet, so we'll do individual operations.
				// This is fine for now as it's a internal background process.
				if err := q.redis.LPush(ctx, targetKey, jobBody); err == nil {
					_ = q.redis.ZRem(ctx, delayedKey, jobBody)
				}
			}
		}
	}
}
