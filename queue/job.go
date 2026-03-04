package queue

import "time"

// Job is the interface that background jobs must implement.
type Job interface {
	// Handle contains the actual job logic.
	Handle() error
	// Retries returns the maximum number of retries before failing.
	Retries() int
	// Queue returns the name of the queue to push this job to.
	Queue() string
	// Backoff returns the duration to wait before retrying.
	Backoff() time.Duration
}

// BaseJob is a struct that jobs can embed to get default settings.
type BaseJob struct{}

// Retries defaults to 3.
func (j *BaseJob) Retries() int {
	return 3
}

// Queue defaults to "default".
func (j *BaseJob) Queue() string {
	return "default"
}

// Backoff defaults to 5 seconds.
func (j *BaseJob) Backoff() time.Duration {
	return 5 * time.Second
}
