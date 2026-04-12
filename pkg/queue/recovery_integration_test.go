package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestJobRecoveryIntegration(t *testing.T) {
	ctx := context.Background()

	// Use miniredis instead of testcontainers
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	// Ensure redis is up
	require.NoError(t, client.Ping(ctx).Err())

	// We create a queue and enqueue a job. Then we manually read it to put it into the PEL (Pending Entries List),
	// but we DON'T ack it. This simulates a crash while processing.
	q := NewRedisQueue(client, "testprefix", nil)

	job := &testRecoveryJob{}
	err = q.Enqueue(ctx, job)
	require.NoError(t, err)

	stream := streamKey("testprefix", "default")
	group := consumerGroupName("testprefix", "default")

	// Ensure group exists
	err = ensureConsumerGroup(ctx, client, stream, group)
	require.NoError(t, err)

	// Read one to put in PEL
	_, err = client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: "crashing_consumer",
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    time.Millisecond,
	}).Result()
	require.NoError(t, err)

	// Verify it's in PEL
	pending, err := client.XPending(ctx, stream, group).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), pending.Count)

	// Override idle time for testing
	originalIdle := redisPendingRecoveryIdle
	redisPendingRecoveryIdle = 1 * time.Millisecond
	defer func() { redisPendingRecoveryIdle = originalIdle }()

	// Sleep briefly to ensure the job exceeds the 1ms idle time
	time.Sleep(10 * time.Millisecond)

	// Now start a new worker to recover the job
	worker := NewRedisWorker(client, "testprefix", []string{"default"}, nil)

	var handled int32
	worker.Register("testRecoveryJob", func() Job {
		return &testRecoveryJob{handled: &handled}
	})

	ctxWorker, cancelWorker := context.WithCancel(ctx)
	err = worker.Start(ctxWorker)
	require.NoError(t, err)

	// Wait a moment for recovery to process
	time.Sleep(100 * time.Millisecond)

	cancelWorker()
	_ = worker.Stop(context.Background())

	require.Equal(t, int32(1), atomic.LoadInt32(&handled), "Job should have been recovered and handled")

	// Ensure PEL is empty after recovery
	pendingAfter, err := client.XPending(ctx, stream, group).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), pendingAfter.Count, "PEL should be empty after successful recovery")
}

type testRecoveryJob struct {
	BaseJob
	handled *int32
}

func (j *testRecoveryJob) Handle(ctx context.Context) error {
	atomic.AddInt32(j.handled, 1)
	return nil
}
