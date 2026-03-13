package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistributedCircuitBreaker(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer rdb.Close()

	cb := NewDistributedCircuitBreaker(rdb, "test-service", DistributedCircuitBreakerOptions{
		MaxFailures:  2,
		ResetTimeout: 100 * time.Millisecond,
	})

	ctx := context.Background()

	// 1. Initially Closed
	status, err := cb.Status(ctx)
	require.NoError(t, err)
	assert.Equal(t, "CLOSED", status)

	err = cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)

	// 2. Trip the circuit
	err = cb.Execute(ctx, func() error { return errors.New("fail") })
	assert.Error(t, err)

	err = cb.Execute(ctx, func() error { return errors.New("fail") })
	assert.Error(t, err)

	status, err = cb.Status(ctx)
	require.NoError(t, err)
	assert.Equal(t, "OPEN", status)

	// 3. Reject requests while open
	err = cb.Execute(ctx, func() error { return nil })
	assert.Equal(t, ErrCircuitOpen, err)

	// 4. Move to Half-Open after timeout
	time.Sleep(150 * time.Millisecond)

	err = cb.Execute(ctx, func() error { return nil })
	assert.NoError(t, err)

	status, err = cb.Status(ctx)
	require.NoError(t, err)
	assert.Equal(t, "CLOSED", status)
}

func TestDistributedCircuitBreaker_HalfOpenFailure(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer rdb.Close()

	cb := NewDistributedCircuitBreaker(rdb, "test-failure", DistributedCircuitBreakerOptions{
		MaxFailures:  1,
		ResetTimeout: 100 * time.Millisecond,
	})

	ctx := context.Background()

	// Trip
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })
	status, _ := cb.Status(ctx)
	assert.Equal(t, "OPEN", status)

	// Wait for reset
	time.Sleep(150 * time.Millisecond)

	// Fail in Half-Open should immediately re-open
	err = cb.Execute(ctx, func() error { return errors.New("fail again") })
	assert.Error(t, err)
	assert.NotEqual(t, ErrCircuitOpen, err)

	status, _ = cb.Status(ctx)
	assert.Equal(t, "OPEN", status)
}
