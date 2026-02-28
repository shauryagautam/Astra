package contracts

import "context"

// JobContract represents a background job.
type JobContract interface {
	// Execute runs the job logic.
	Execute(ctx context.Context) error

	// DisplayName returns a human-readable name for the job.
	DisplayName() string

	// Tries returns the maximum number of attempts for this job.
	Tries() int

	// Backoff returns the delay in seconds between retries.
	Backoff() int
}

// QueueContract defines the queue manager.
// Mirrors AdonisJS's Queue provider.
type QueueContract interface {
	// Push adds a job to the queue.
	Push(job JobContract) error

	// PushRaw adds a raw payload to the queue for a given job name.
	PushRaw(jobName string, data any) error

	// Later adds a job to be executed after a delay.
	Later(delay int, job JobContract) error

	// Process starts a worker for a specific queue.
	Process(queueName string) error
}

// JobHandler is a function that processes a raw job.
type JobHandler func(data []byte) error

// JobRegistry is used to map job names to handlers.
type JobRegistry interface {
	// Register adds a job handler to the registry.
	Register(name string, handler JobHandler)

	// Get retrieves a handler by name.
	Get(name string) (JobHandler, bool)
}
