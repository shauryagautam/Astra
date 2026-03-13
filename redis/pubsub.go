package redis

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

// Publish sends a JSON-serialized message to a Redis channel.
func (c *Client) Publish(ctx context.Context, channel string, message any) error {
	data, err := sonic.Marshal(message)
	if err != nil {
		return fmt.Errorf("redis: failed to marshal message: %w", err)
	}
	return c.UniversalClient.Publish(ctx, channel, data).Err()
}

// Subscribe listens for messages on a channel and handles them with a callback.
func (c *Client) Subscribe(ctx context.Context, channel string, handler func(payload []byte) error) (*redis.PubSub, error) {
	pubsub := c.UniversalClient.Subscribe(ctx, channel)

	go func() {
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				if err := pubsub.Close(); err != nil {
					// Ignore close error
				}
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if err := handler([]byte(msg.Payload)); err != nil {
					// In a real framework, we'd log this error using the internal logger.
					fmt.Printf("redis: sub handler error on channel %s: %v\n", channel, err)
				}
			}
		}
	}()

	return pubsub, nil
}

// PSubscribe listens for messages on channels matching a pattern.
func (c *Client) PSubscribe(ctx context.Context, pattern string, handler func(channel string, payload []byte) error) (*redis.PubSub, error) {
	pubsub := c.UniversalClient.PSubscribe(ctx, pattern)

	go func() {
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				if err := pubsub.Close(); err != nil {
					// Ignore close error
				}
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if err := handler(msg.Channel, []byte(msg.Payload)); err != nil {
					fmt.Printf("redis: psub handler error on pattern %s: %v\n", pattern, err)
				}
			}
		}
	}()

	return pubsub, nil
}
