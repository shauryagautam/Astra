package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/shaurya/adonis/contracts"
)

// RedisQueue implements the QueueContract using Redis via the contracts interface.
type RedisQueue struct {
	redis    contracts.RedisConnectionContract
	registry contracts.JobRegistry
	prefix   string
}

// NewRedisQueue creates a new Redis-backed queue manager.
func NewRedisQueue(r contracts.RedisConnectionContract, registry contracts.JobRegistry) *RedisQueue {
	return &RedisQueue{
		redis:    r,
		registry: registry,
		prefix:   "adonis:queue:",
	}
}

// JobPayload represents the data stored in Redis.
type JobPayload struct {
	Name      string    `json:"name"`
	Data      []byte    `json:"data"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
}

// Push adds a job to the default queue.
func (q *RedisQueue) Push(job contracts.JobContract) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return q.PushRaw(job.DisplayName(), data)
}

// PushRaw adds a raw payload to the queue.
func (q *RedisQueue) PushRaw(jobName string, data any) error {
	var rawData []byte
	var err error

	switch v := data.(type) {
	case []byte:
		rawData = v
	default:
		rawData, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal raw data: %w", err)
		}
	}

	payload := JobPayload{
		Name:      jobName,
		Data:      rawData,
		Attempts:  0,
		CreatedAt: time.Now(),
	}

	payloadBytes, _ := json.Marshal(payload)

	// Default queue name is "default"
	return q.redis.LPush(context.Background(), q.prefix+"default", payloadBytes)
}

// Later adds a job to be executed after a delay using a sorted set.
func (q *RedisQueue) Later(delay int, job contracts.JobContract) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	payload := JobPayload{
		Name:      job.DisplayName(),
		Data:      data,
		Attempts:  0,
		CreatedAt: time.Now(),
	}

	payloadBytes, _ := json.Marshal(payload)
	executeAt := float64(time.Now().Add(time.Duration(delay) * time.Second).Unix())

	return q.redis.ZAdd(context.Background(), q.prefix+"delayed", executeAt, payloadBytes)
}

// Registry implementation
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]contracts.JobHandler
}

// NewRegistry creates a new job registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]contracts.JobHandler),
	}
}

func (r *Registry) Register(name string, handler contracts.JobHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

func (r *Registry) Get(name string) (contracts.JobHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}
