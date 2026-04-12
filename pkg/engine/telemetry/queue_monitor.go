package telemetry

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// QueueStat holds aggregate stats for a single named queue.
type QueueStat struct {
	Name       string `json:"name"`
	Pending    int64  `json:"pending"`    // jobs waiting to be processed
	Processing int64  `json:"processing"` // jobs currently in-flight (reserved)
	Failed     int64  `json:"failed"`     // jobs in the dead-letter list
	Scheduled  int64  `json:"scheduled"`  // jobs in the delayed sorted-set
	LastPolled string `json:"last_polled"`
}

// QueueMonitor introspects the Redis data structures used by the Astra Queue
// package to provide real-time visibility into queue depth and failures.
//
// It supports three queue key patterns (all configurable via KeyPrefix):
//   - List  <prefix>:<name>        — main pending queue (LLEN)
//   - List  <prefix>:<name>:retry  — in-flight / processing
//   - List  <prefix>:<name>:failed — dead-letter list
//   - ZSet  <prefix>:<name>:delayed — scheduled future jobs (ZCARD)
type QueueMonitor struct {
	mu        sync.RWMutex
	client    goredis.UniversalClient
	keyPrefix string
	queues    []string // registered queue names to monitor
}

// NewQueueMonitor creates a QueueMonitor backed by the provided Redis client.
// keyPrefix should match the prefix used by the queue package (default: "astra:queue").
// queueNames is the list of queue names to monitor (e.g. ["default", "mail", "notifications"]).
func NewQueueMonitor(client goredis.UniversalClient, keyPrefix string, queueNames ...string) *QueueMonitor {
	if keyPrefix == "" {
		keyPrefix = "astra:queue"
	}
	return &QueueMonitor{
		client:    client,
		keyPrefix: strings.TrimRight(keyPrefix, ":"),
		queues:    queueNames,
	}
}

// RegisterQueue adds a queue name to the monitor's watch list.
func (m *QueueMonitor) RegisterQueue(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, q := range m.queues {
		if q == name {
			return
		}
	}
	m.queues = append(m.queues, name)
}

// QueueStats polls Redis and returns current stats for all registered queues.
// The call is synchronous and typically completes in < 1ms for local Redis.
func (m *QueueMonitor) QueueStats(ctx context.Context) ([]QueueStat, error) {
	m.mu.RLock()
	names := append([]string(nil), m.queues...)
	m.mu.RUnlock()

	if len(names) == 0 {
		// Auto-discover: scan for keys matching the prefix pattern.
		discovered, err := m.discover(ctx)
		if err == nil {
			names = discovered
		}
	}

	stats := make([]QueueStat, 0, len(names))
	now := time.Now().Format(time.RFC3339)

	for _, name := range names {
		stat, err := m.statFor(ctx, name)
		if err != nil {
			// Non-fatal: include the queue with -1 to indicate error.
			stats = append(stats, QueueStat{Name: name, Pending: -1, LastPolled: now})
			continue
		}
		stat.LastPolled = now
		stats = append(stats, stat)
	}

	return stats, nil
}

func (m *QueueMonitor) statFor(ctx context.Context, name string) (QueueStat, error) {
	prefix := m.keyPrefix + ":" + name

	pipe := m.client.Pipeline()
	pendingCmd := pipe.LLen(ctx, prefix)
	processingCmd := pipe.LLen(ctx, prefix+":retry")
	failedCmd := pipe.LLen(ctx, prefix+":failed")
	scheduledCmd := pipe.ZCard(ctx, prefix+":delayed")
	_, err := pipe.Exec(ctx)
	if err != nil && err != goredis.Nil {
		return QueueStat{}, fmt.Errorf("queue monitor: pipeline exec: %w", err)
	}

	return QueueStat{
		Name:       name,
		Pending:    pendingCmd.Val(),
		Processing: processingCmd.Val(),
		Failed:     failedCmd.Val(),
		Scheduled:  scheduledCmd.Val(),
	}, nil
}

// discover scans Redis for keys matching the queue prefix and extracts queue names.
func (m *QueueMonitor) discover(ctx context.Context) ([]string, error) {
	pattern := m.keyPrefix + ":*"
	var cursor uint64
	seen := make(map[string]struct{})

	for {
		keys, next, err := m.client.Scan(ctx, cursor, pattern, 50).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			after := strings.TrimPrefix(k, m.keyPrefix+":")
			// Strip suffixes like :retry, :failed, :delayed
			parts := strings.SplitN(after, ":", 2)
			name := parts[0]
			if name != "" && !strings.ContainsAny(name, " \t\n") {
				seen[name] = struct{}{}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	return names, nil
}

// TotalFailed returns the total number of failed jobs across all queues.
func (m *QueueMonitor) TotalFailed(ctx context.Context) int64 {
	stats, err := m.QueueStats(ctx)
	if err != nil {
		return -1
	}
	var total int64
	for _, s := range stats {
		if s.Failed > 0 {
			total += s.Failed
		}
	}
	return total
}

// RetryFailed moves all failed jobs back to the pending queue for re-processing.
// queueName must match a registered queue. Passing "" retries all queues.
func (m *QueueMonitor) RetryFailed(ctx context.Context, queueName string) (int64, error) {
	m.mu.RLock()
	names := append([]string(nil), m.queues...)
	m.mu.RUnlock()

	var retried int64
	for _, name := range names {
		if queueName != "" && name != queueName {
			continue
		}
		failedKey := m.keyPrefix + ":" + name + ":failed"
		pendingKey := m.keyPrefix + ":" + name

		// Pop all from failed, push to pending.
		for {
			val, err := m.client.RPop(ctx, failedKey).Result()
			if err == goredis.Nil {
				break
			}
			if err != nil {
				return retried, fmt.Errorf("queue monitor: retry failed jobs: %w", err)
			}
			if err := m.client.LPush(ctx, pendingKey, val).Err(); err != nil {
				return retried, err
			}
			retried++
		}
	}

	return retried, nil
}

// PurgeQueue removes all pending jobs from a specific queue (destructive!).
func (m *QueueMonitor) PurgeQueue(ctx context.Context, name string) (int64, error) {
	key := m.keyPrefix + ":" + name
	count, err := m.client.LLen(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if err := m.client.Del(ctx, key).Err(); err != nil {
		return 0, err
	}
	return count, nil
}

// ScheduledJobs returns the first n jobs from the delayed sorted-set for a queue.
func (m *QueueMonitor) ScheduledJobs(ctx context.Context, queueName string, n int) ([]ScheduledJob, error) {
	key := m.keyPrefix + ":" + queueName + ":delayed"
	results, err := m.client.ZRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("queue monitor: scheduled jobs: %w", err)
	}

	jobs := make([]ScheduledJob, 0, len(results))
	for _, z := range results {
		payload, _ := z.Member.(string)
		ts := int64(z.Score)
		jobs = append(jobs, ScheduledJob{
			Payload:     payload,
			RunAt:       time.Unix(ts, 0),
			RunAtUnix:   strconv.FormatInt(ts, 10),
		})
	}
	return jobs, nil
}

// ScheduledJob represents a single entry in the delayed sorted-set.
type ScheduledJob struct {
	Payload   string    `json:"payload"`
	RunAt     time.Time `json:"run_at"`
	RunAtUnix string    `json:"run_at_unix"`
}
