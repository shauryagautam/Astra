package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// XAdd adds a message to a Redis stream.
func (c *Client) XAdd(ctx context.Context, stream string, values map[string]any) (string, error) {
	return c.UniversalClient.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: values,
	}).Result()
}

// XRead reads messages from one or more streams.
func (c *Client) XRead(ctx context.Context, streams []string, ids []string, count int64, block time.Duration) ([]redis.XStream, error) {
	return c.UniversalClient.XRead(ctx, &redis.XReadArgs{
		Streams: append(streams, ids...),
		Count:   count,
		Block:   block,
	}).Result()
}

// XGroupCreate creates a consumer group for a stream.
func (c *Client) XGroupCreate(ctx context.Context, stream, group, start string) error {
	return c.UniversalClient.XGroupCreate(ctx, stream, group, start).Err()
}

// XReadGroup reads messages from a stream using a consumer group.
func (c *Client) XReadGroup(ctx context.Context, group, consumer string, streams []string, ids []string, count int64, block time.Duration) ([]redis.XStream, error) {
	return c.UniversalClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  append(streams, ids...),
		Count:    count,
		Block:    block,
	}).Result()
}

// XAck acknowledges one or more messages in a consumer group.
func (c *Client) XAck(ctx context.Context, stream, group string, ids ...string) error {
	return c.UniversalClient.XAck(ctx, stream, group, ids...).Err()
}
