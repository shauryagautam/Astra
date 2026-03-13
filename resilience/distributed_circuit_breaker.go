package resilience

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedCircuitBreaker implements the circuit breaker pattern using Redis for state.
type DistributedCircuitBreaker struct {
	client       redis.UniversalClient
	key          string
	maxFailures  int
	resetTimeout time.Duration
}

// DistributedCircuitBreakerOptions defines configuration for the distributed breaker.
type DistributedCircuitBreakerOptions struct {
	MaxFailures  int
	ResetTimeout time.Duration
}

var (
	// State constants used in Redis
	stateClosed   = "CLOSED"
	stateOpen     = "OPEN"
	stateHalfOpen = "HALF_OPEN"
)

// NewDistributedCircuitBreaker creates a new Redis-backed circuit breaker.
func NewDistributedCircuitBreaker(client redis.UniversalClient, key string, opts DistributedCircuitBreakerOptions) *DistributedCircuitBreaker {
	if opts.MaxFailures == 0 {
		opts.MaxFailures = 5
	}
	if opts.ResetTimeout == 0 {
		opts.ResetTimeout = 30 * time.Second
	}

	return &DistributedCircuitBreaker{
		client:       client,
		key:          "circuit_breaker:" + key,
		maxFailures:  opts.MaxFailures,
		resetTimeout: opts.ResetTimeout,
	}
}

// Lua script for atomic state checks and transitions
var recordResultScript = redis.NewScript(`
local key = KEYS[1]
local max_failures = tonumber(ARGV[1])
local reset_timeout_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local success = tonumber(ARGV[4]) == 1

local state = redis.call("HGET", key, "state") or "CLOSED"
local fail_count = tonumber(redis.call("HGET", key, "fail_count") or "0")
local opened_at = tonumber(redis.call("HGET", key, "opened_at") or "0")

if success then
    if state == "HALF_OPEN" or state == "OPEN" then
        redis.call("HMSET", key, "state", "CLOSED", "fail_count", "0")
    elseif state == "CLOSED" then
        redis.call("HSET", key, "fail_count", "0")
    end
    return 1
else
    fail_count = fail_count + 1
    redis.call("HSET", key, "fail_count", fail_count)

    if state == "HALF_OPEN" or fail_count >= max_failures then
        redis.call("HMSET", key, "state", "OPEN", "opened_at", now_ms)
        return 0
    end
    return 1
end
`)

var allowRequestScript = redis.NewScript(`
local key = KEYS[1]
local reset_timeout_ms = tonumber(ARGV[1])
local now_ms = tonumber(ARGV[2])

local state = redis.call("HGET", key, "state") or "CLOSED"
local opened_at = tonumber(redis.call("HGET", key, "opened_at") or "0")

if state == "OPEN" then
    if now_ms - opened_at > reset_timeout_ms then
        redis.call("HSET", key, "state", "HALF_OPEN")
        return 1
    end
    return 0
end

return 1
`)

// Execute wraps a function call with distributed circuit breaker logic.
func (cb *DistributedCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	allowed, err := allowRequestScript.Run(ctx, cb.client, []string{cb.key}, cb.resetTimeout.Milliseconds(), time.Now().UnixMilli()).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("astra/resilience: redis error: %w", err)
	}

	if allowed == 0 {
		return ErrCircuitOpen
	}

	fnErr := fn()

	_, err = recordResultScript.Run(ctx, cb.client, []string{cb.key}, cb.maxFailures, cb.resetTimeout.Milliseconds(), time.Now().UnixMilli(), fnErr == nil).Result()
	if err != nil {
		// Log error but don't fail the request if Redis is down after execution
		return fnErr
	}

	return fnErr
}

// Status returns the current state from Redis.
func (cb *DistributedCircuitBreaker) Status(ctx context.Context) (string, error) {
	state, err := cb.client.HGet(ctx, cb.key, "state").Result()
	if errors.Is(err, redis.Nil) {
		return "CLOSED", nil
	}
	return state, err
}
