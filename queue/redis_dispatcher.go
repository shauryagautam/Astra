package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/astraframework/astra/json"
	"github.com/redis/go-redis/v9"
)

// RedisDispatcher pushes jobs onto a Redis-backed queue.
type RedisDispatcher struct {
	client redis.UniversalClient
	queue  *RedisQueue
	prefix string
}

// NewRedisDispatcher creates a new Redis-backed dispatcher.
func NewRedisDispatcher(client redis.UniversalClient, prefix string) *RedisDispatcher {
	queue := NewRedisQueue(client, prefix, nil)
	return &RedisDispatcher{
		client: client,
		queue:  queue,
		prefix: normalizeQueuePrefix(prefix),
	}
}

// Dispatch pushes a job for immediate processing.
func (d *RedisDispatcher) Dispatch(ctx context.Context, job Job, name string) error {
	return d.queue.enqueue(ctx, name, job, 0)
}

// DispatchUnique pushes a job only when the uniqueness lock is available.
func (d *RedisDispatcher) DispatchUnique(ctx context.Context, job Job, name string, ttl time.Duration) error {
	if d.client == nil {
		return errNilRedisClient
	}
	key := fmt.Sprintf("%s:queue:unique:%s:%s", d.prefix, name, job.Queue())
	ok, err := d.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return fmt.Errorf("astra/queue: %w", err)
	}
	if !ok {
		return nil
	}
	return d.Dispatch(ctx, job, name)
}

// DispatchIn pushes a job to the delayed queue.
func (d *RedisDispatcher) DispatchIn(ctx context.Context, job Job, name string, delay time.Duration) error {
	return d.DispatchAt(ctx, job, name, time.Now().Add(delay))
}

// DispatchAt pushes a job to run at a specific time.
func (d *RedisDispatcher) DispatchAt(ctx context.Context, job Job, name string, at time.Time) error {
	envelope, err := newQueueEnvelope(ctx, name, job, 0)
	if err != nil {
		return err
	}
	body, err := json.Marshal(delayedEnvelope{RunAt: at.UTC(), Job: envelope})
	if err != nil {
		return fmt.Errorf("astra/queue: %w", err)
	}
	return d.client.ZAdd(ctx, delayedQueueKey(d.prefix), redis.Z{
		Score:  float64(at.Unix()),
		Member: body,
	}).Err()
}
