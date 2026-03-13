package redis

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LeaderElector provides distributed leader election.
type LeaderElector struct {
	client *Client
	name   string
	id     string
}

// NewLeaderElector creates a new LeaderElector instance.
func (c *Client) NewLeaderElector(name string) *LeaderElector {
	return &LeaderElector{
		client: c,
		name:   "leader:" + name,
		id:     uuid.New().String(),
	}
}

// Elect attempts to become the leader and maintain leadership.
// It calls the leaderFunc when it becomes the leader.
func (le *LeaderElector) Elect(ctx context.Context, ttl time.Duration, leaderFunc func(ctx context.Context)) {
	ticker := time.NewTicker(ttl / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Try to acquire leadership
			acquired, err := le.client.UniversalClient.SetNX(ctx, le.name, le.id, ttl).Result()
			if err != nil {
				continue
			}

			if acquired {
				// We became the leader!
				leaderCtx, cancel := context.WithCancel(ctx)
				go func() {
					// Periodically renew leadership
					renewTicker := time.NewTicker(ttl / 2)
					defer renewTicker.Stop()
					for {
						select {
						case <-leaderCtx.Done():
							return
						case <-renewTicker.C:
							ok, err := le.client.UniversalClient.Eval(ctx, `
								if redis.call("get", KEYS[1]) == ARGV[1] then
									return redis.call("pexpire", KEYS[1], ARGV[2])
								else
									return 0
								end
							`, []string{le.name}, le.id, ttl.Milliseconds()).Result()
							if err != nil || ok == int64(0) {
								cancel()
								return
							}
						}
					}
				}()

				// Run the leader function
				leaderFunc(leaderCtx)
				cancel()
			} else {
				// Check if we are already the leader (e.g., after a reconnect)
				val, err := le.client.UniversalClient.Get(ctx, le.name).Result()
				if err == nil && val == le.id {
					// We are still the leader, just continue (auto-renewal logic will handle it)
					continue
				}
			}
		}
	}
}
