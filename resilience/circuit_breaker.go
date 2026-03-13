package resilience

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// CircuitState represents the current state of the circuit breaker.
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// StateStore defines the interface for persisting circuit breaker state.
type StateStore interface {
	Get(ctx context.Context, key string) (CircuitState, int, time.Time, error)
	Set(ctx context.Context, key string, state CircuitState, failCount int, openedAt time.Time, ttl time.Duration) error
}

// InMemoryStore is the default node-local state store.
type InMemoryStore struct {
	mu        sync.RWMutex
	state     CircuitState
	failCount int
	openedAt  time.Time
}

func (s *InMemoryStore) Get(_ context.Context, _ string) (CircuitState, int, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state, s.failCount, s.openedAt, nil
}

func (s *InMemoryStore) Set(_ context.Context, _ string, state CircuitState, failCount int, openedAt time.Time, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.failCount = failCount
	s.openedAt = openedAt
	return nil
}

// RedisStateStore is a Redis-backed state store for distributed circuit breakers.
type RedisStateStore struct {
	client goredis.UniversalClient
	prefix string
}

func NewRedisStateStore(client goredis.UniversalClient, prefix string) *RedisStateStore {
	if prefix == "" {
		prefix = "astra:cb:"
	}
	if !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}
	return &RedisStateStore{
		client: client,
		prefix: prefix,
	}
}

type redisCBState struct {
	State     CircuitState `json:"state"`
	FailCount int          `json:"fail_count"`
	OpenedAt  time.Time    `json:"opened_at"`
}

func (s *RedisStateStore) Get(ctx context.Context, key string) (CircuitState, int, time.Time, error) {
	val, err := s.client.Get(ctx, s.prefix+key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return StateClosed, 0, time.Time{}, nil
		}
		return StateClosed, 0, time.Time{}, err
	}

	var state redisCBState
	if err := json.Unmarshal(val, &state); err != nil {
		return StateClosed, 0, time.Time{}, err
	}

	return state.State, state.FailCount, state.OpenedAt, nil
}

func (s *RedisStateStore) Set(ctx context.Context, key string, state CircuitState, failCount int, openedAt time.Time, ttl time.Duration) error {
	data, err := json.Marshal(redisCBState{
		State:     state,
		FailCount: failCount,
		OpenedAt:  openedAt,
	})
	if err != nil {
		return err
	}

	if ttl <= 0 {
		ttl = 24 * time.Hour // Default long TTL for non-open states
	}

	return s.client.Set(ctx, s.prefix+key, data, ttl).Err()
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu           sync.Mutex
	name         string
	store        StateStore
	maxFailures  int
	resetTimeout time.Duration
}

// NewCircuitBreaker creates a new circuit breaker with default settings.
func NewCircuitBreaker(name string) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		store:        &InMemoryStore{},
		maxFailures:  5,
		resetTimeout: 30 * time.Second,
	}
}

// WithStore sets the state store for the circuit breaker.
func (cb *CircuitBreaker) WithStore(store StateStore) *CircuitBreaker {
	cb.store = store
	return cb
}

// Execute wraps a function call with circuit breaker logic.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.allowRequest(ctx) {
		return ErrCircuitOpen
	}

	err := fn()
	cb.trackResult(ctx, err)
	return err
}

func (cb *CircuitBreaker) allowRequest(ctx context.Context) bool {
	state, _, openedAt, err := cb.store.Get(ctx, cb.name)
	if err != nil {
		return true // Fail open on store errors
	}

	if state == StateOpen {
		if time.Since(openedAt) > cb.resetTimeout {
			_ = cb.store.Set(ctx, cb.name, StateHalfOpen, 0, time.Time{}, cb.resetTimeout*2)
			return true
		}
		return false
	}

	return true
}

func (cb *CircuitBreaker) trackResult(ctx context.Context, err error) {
	state, failCount, _, storeErr := cb.store.Get(ctx, cb.name)
	if storeErr != nil {
		return
	}

	if err == nil {
		if state == StateHalfOpen {
			_ = cb.store.Set(ctx, cb.name, StateClosed, 0, time.Time{}, 0)
		}
		return
	}

	failCount++

	if state == StateHalfOpen || failCount >= cb.maxFailures {
		_ = cb.store.Set(ctx, cb.name, StateOpen, failCount, time.Now(), cb.resetTimeout*2)
	} else {
		_ = cb.store.Set(ctx, cb.name, state, failCount, time.Time{}, cb.resetTimeout*2)
	}
}

// Status returns a string representation of the current state.
func (cb *CircuitBreaker) Status(ctx context.Context) string {
	state, _, openedAt, _ := cb.store.Get(ctx, cb.name)

	switch state {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		until := "unknown"
		if !openedAt.IsZero() {
			until = openedAt.Add(cb.resetTimeout).Format("15:04:05")
		}
		return fmt.Sprintf("OPEN (until %v)", until)
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "UNKNOWN"
	}
}
