package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Dispatcher handles pushing jobs to the queue.
type Dispatcher struct {
	client *redis.Client
	prefix string
}

// NewDispatcher creates a new Job dispatcher.
func NewDispatcher(client *redis.Client, prefix string) *Dispatcher {
	return &Dispatcher{
		client: client,
		prefix: prefix,
	}
}

// payload represents the serialized job.
type payload struct {
	Name      string `json:"name"`
	Data      []byte `json:"data"`
	Retries   int    `json:"retries"`
	Attempts  int    `json:"attempts"`
	CreatedAt int64  `json:"created_at"`
}

// Dispatch pushes a job to its configured queue immediately.
func (d *Dispatcher) Dispatch(ctx context.Context, job Job, name string) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	p := payload{
		Name:      name,
		Data:      data,
		Retries:   job.Retries(),
		Attempts:  0,
		CreatedAt: time.Now().Unix(),
	}

	bytes, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	queueKey := d.prefix + job.Queue()
	return d.client.LPush(ctx, queueKey, bytes).Err()
}

// DispatchUnique pushes a job to the queue only if it's unique within the ttl.
// It uses Redis SETNX to enforce uniqueness for the given duration.
func (d *Dispatcher) DispatchUnique(ctx context.Context, job Job, name string, ttl time.Duration) error {
	uniqueKey := d.prefix + "unique:" + name + ":" + job.Queue()
	ok, err := d.client.SetNX(ctx, uniqueKey, "1", ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return nil // Already dispatched and still protected by TTL
	}
	return d.Dispatch(ctx, job, name)
}

// DispatchIn pushes a job to the delayed queue to be processed after the duration.
func (d *Dispatcher) DispatchIn(ctx context.Context, job Job, name string, delay time.Duration) error {
	return d.DispatchAt(ctx, job, name, time.Now().Add(delay))
}

// DispatchAt pushes a job to the delayed queue to be processed at the specified time.
func (d *Dispatcher) DispatchAt(ctx context.Context, job Job, name string, at time.Time) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	p := payload{
		Name:      name,
		Data:      data,
		Retries:   job.Retries(),
		Attempts:  0,
		CreatedAt: time.Now().Unix(),
	}

	bytes, err := json.Marshal(p)
	if err != nil {
		return err
	}

	delayedKey := d.prefix + "delayed:" + job.Queue()
	return d.client.ZAdd(ctx, delayedKey, redis.Z{
		Score:  float64(at.Unix()),
		Member: bytes,
	}).Err()
}
