package queue

import (
	"context"
	"time"
)

const (
	defaultQueueName    = "default"
	defaultMaxRetries   = 3
	defaultJobTimeout   = 30 * time.Second
	defaultQueuePrefix  = "astra"
	defaultPollInterval = time.Second
)

// Queue stores background jobs for later execution.
type Queue interface {
	// Enqueue stores a job for immediate execution.
	Enqueue(ctx context.Context, job Job) error
	// EnqueueIn stores a job for execution after the provided delay.
	EnqueueIn(ctx context.Context, job Job, delay time.Duration) error
	// EnqueueAt stores a job for execution at the provided time.
	EnqueueAt(ctx context.Context, job Job, at time.Time) error
	// Size reports the number of ready jobs for a queue.
	Size(ctx context.Context, queue string) (int64, error)
	// Purge removes all pending jobs for a queue.
	Purge(ctx context.Context, queue string) error
}

// Job is the interface that background jobs must implement.
type Job interface {
	// Handle contains the actual job logic.
	Handle(ctx context.Context) error
	// OnFailure is invoked when the job permanently fails.
	OnFailure(ctx context.Context, err error)
	// MaxRetries returns the maximum number of attempts before failing.
	MaxRetries() int
	// Queue returns the queue name to push the job to.
	Queue() string
	// Timeout returns the maximum execution time for a single attempt.
	Timeout() time.Duration
}

// BaseJob is a helper that provides production-safe defaults.
type BaseJob struct{}

// OnFailure is a no-op by default.
func (j *BaseJob) OnFailure(ctx context.Context, err error) {}

// MaxRetries defaults to three attempts.
func (j *BaseJob) MaxRetries() int {
	return defaultMaxRetries
}

// Queue defaults to the "default" queue.
func (j *BaseJob) Queue() string {
	return defaultQueueName
}

// Timeout defaults to 30 seconds.
func (j *BaseJob) Timeout() time.Duration {
	return defaultJobTimeout
}
