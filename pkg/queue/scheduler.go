package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

// ScheduledJob holds metadata about a registered cron job.
type ScheduledJob struct {
	ID      cron.EntryID
	Name    string
	Spec    string
	NextRun time.Time
	PrevRun time.Time
}

// Scheduler runs named cron jobs with optional Redis distributed locking
// to prevent duplicate runs across multiple application instances.
// It also handles moving delayed jobs to ready queues.
type Scheduler struct {
	client  redis.UniversalClient
	queue   *RedisQueue
	prefix  string
	cron    *cron.Cron
	entries []ScheduledJob
}

// NewScheduler creates a new scheduler.
func NewScheduler(client redis.UniversalClient, prefix string, queue *RedisQueue) *Scheduler {
	return &Scheduler{
		client: client,
		queue:  queue,
		prefix: prefix,
		cron:   cron.New(cron.WithSeconds()),
	}
}

// Register adds a named cron job. If a Redis client is configured, a distributed
// lock is acquired before each run to prevent concurrent execution across instances.
func (s *Scheduler) Register(name, spec string, fn func()) (cron.EntryID, error) {
	wrapped := func() {
		if s.client != nil {
			lockKey := s.prefix + "sched:lock:" + name
			// Try to acquire a 55-second lock (shorter than most cron intervals)
			ok, err := s.client.SetNX(context.Background(), lockKey, "1", 55*time.Second).Result()
			if err != nil || !ok {
				// Another instance is running this job, skip
				return
			}
			defer s.client.Del(context.Background(), lockKey)
		}
		fn()
	}

	id, err := s.cron.AddFunc(spec, wrapped)
	if err != nil {
		return 0, fmt.Errorf("scheduler: invalid cron spec %q for job %q: %w", spec, name, err)
	}

	s.entries = append(s.entries, ScheduledJob{
		ID:   id,
		Name: name,
		Spec: spec,
	})
	return id, nil
}

// Schedule adds an anonymous cron job (legacy API, no lock protection).
func (s *Scheduler) Schedule(spec string, cmd func()) (cron.EntryID, error) {
	return s.cron.AddFunc(spec, cmd)
}

// List returns metadata for all registered named jobs including their next run time.
func (s *Scheduler) List() []ScheduledJob {
	result := make([]ScheduledJob, len(s.entries))
	for i, e := range s.entries {
		entry := s.cron.Entry(e.ID)
		result[i] = ScheduledJob{
			ID:      e.ID,
			Name:    e.Name,
			Spec:    e.Spec,
			NextRun: entry.Next,
			PrevRun: entry.Prev,
		}
	}
	return result
}

// Start begins the cron scheduler and delayed job polling.
func (s *Scheduler) Start(ctx context.Context) error {
	s.cron.Start()

	// Background goroutine to move delayed jobs to ready queues
	if s.queue != nil {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Delegate delayed job promotion to RedisQueue
					_ = s.queue.PromoteReady(ctx)
				}
			}
		}()
	}

	return nil
}

// Stop stops the scheduler gracefully.
func (s *Scheduler) Stop(ctx context.Context) error {
	stopCtx := s.cron.Stop()
	// Wait for running jobs to finish
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
	}
	return nil
}
