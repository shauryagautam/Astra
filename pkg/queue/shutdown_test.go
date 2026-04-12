package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type sleepJob struct {
	BaseJob
	sleep time.Duration
	done  *atomic.Bool
}

func (j *sleepJob) Handle(ctx context.Context) error {
	time.Sleep(j.sleep)
	if j.done != nil {
		j.done.Store(true)
	}
	return nil
}

func (j *sleepJob) Timeout() time.Duration {
	return 5 * time.Second
}

func TestRedisWorker_Stop_Graceful(t *testing.T) {
	worker := NewRedisWorker(nil, "astra", []string{"default"}, nil)

	// Simulate a job in flight
	var jobDone atomic.Bool
	job := &sleepJob{sleep: 100 * time.Millisecond, done: &jobDone}

	worker.wg.Add(1)
	worker.inFlight.Add(1)
	go func() {
		defer worker.wg.Done()
		defer worker.inFlight.Add(-1)
		_ = job.Handle(context.Background())
	}()

	// Start shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := worker.Stop(shutdownCtx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, jobDone.Load(), "In-flight job should have finished")
	assert.True(t, duration >= 100*time.Millisecond, "Should have waited for the job")
}

func TestRedisWorker_Stop_Timeout(t *testing.T) {
	worker := NewRedisWorker(nil, "astra", []string{"default"}, nil)

	// Simulate a very slow job in flight
	var jobDone atomic.Bool
	job := &sleepJob{sleep: 500 * time.Millisecond, done: &jobDone}

	worker.wg.Add(1)
	worker.inFlight.Add(1)
	go func() {
		defer worker.wg.Done()
		defer worker.inFlight.Add(-1)
		_ = job.Handle(context.Background())
	}()

	// Start shutdown with a shorter timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := worker.Stop(shutdownCtx)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.False(t, jobDone.Load(), "Job should NOT have finished due to timeout")
}

func TestRedisWorker_DrainingFlag(t *testing.T) {
	worker := NewRedisWorker(nil, "astra", []string{"default"}, nil)
	assert.False(t, worker.draining.Load())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_ = worker.Stop(ctx)
	assert.True(t, worker.draining.Load(), "Draining flag should be set after Stop")
}
