package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

// StreamsDriver implements the queue using Redis Streams.
type StreamsDriver struct {
	client redis.UniversalClient
	prefix string
}

func NewStreamsDriver(client redis.UniversalClient, prefix string) *StreamsDriver {
	return &StreamsDriver{
		client: client,
		prefix: prefix,
	}
}

// Dispatch pushes a job to a Redis Stream.
func (d *StreamsDriver) Dispatch(ctx context.Context, job Job, name string) error {
	data, err := sonic.Marshal(job)
	if err != nil {
		return fmt.Errorf("queue.Streams.Dispatch: %w", err)
	}

	p := payload{
		Name:      name,
		Data:      data,
		Retries:   job.Retries(),
		Attempts:  0,
		CreatedAt: time.Now().Unix(),
	}

	bytes, err := sonic.Marshal(p)
	if err != nil {
		return fmt.Errorf("queue.Streams.Dispatch: %w", err)
	}

	streamKey := d.prefix + "stream:" + job.Queue()
	return d.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"payload": bytes,
		},
	}).Err()
}
