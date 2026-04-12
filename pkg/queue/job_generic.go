package queue

import (
	"time"
)

// TypedJob is a generic interface for background jobs with typed payloads.
type TypedJob[T any] interface {
	Job
	Data() T
}

// GenericJob is a helper that wraps a payload and common job metadata.
type GenericJob[T any] struct {
	BaseJob
	Payload     T
	JobName     string
	JobQueue    string
	JobMaxRetry int
	JobTimeoutD time.Duration
}

// NewJob creates a new generic job with the provided name and payload.
func NewJob[T any](name string, payload T) *GenericJob[T] {
	return &GenericJob[T]{
		Payload:     payload,
		JobName:     name,
		JobMaxRetry: defaultMaxRetries,
		JobTimeoutD: defaultJobTimeout,
		JobQueue:    defaultQueueName,
	}
}

// WithQueue sets the queue name for the job.
func (j *GenericJob[T]) WithQueue(name string) *GenericJob[T] {
	j.JobQueue = name
	return j
}

// WithMaxRetries sets the maximum retry attempts.
func (j *GenericJob[T]) WithMaxRetries(n int) *GenericJob[T] {
	j.JobMaxRetry = n
	return j
}

// WithTimeout sets the execution timeout.
func (j *GenericJob[T]) WithTimeout(d time.Duration) *GenericJob[T] {
	j.JobTimeoutD = d
	return j
}

// JobType returns the registered name of the job.
func (j *GenericJob[T]) JobType() string {
	return j.JobName
}

// Queue returns the queue name.
func (j *GenericJob[T]) Queue() string {
	if j.JobQueue != "" {
		return j.JobQueue
	}
	return defaultQueueName
}

// MaxRetries returns the max retry attempts.
func (j *GenericJob[T]) MaxRetries() int {
	if j.JobMaxRetry > 0 {
		return j.JobMaxRetry
	}
	return defaultMaxRetries
}

// Timeout returns the execution timeout.
func (j *GenericJob[T]) Timeout() time.Duration {
	if j.JobTimeoutD > 0 {
		return j.JobTimeoutD
	}
	return defaultJobTimeout
}

// PayloadData returns the typed payload.
func (j *GenericJob[T]) Data() T {
	return j.Payload
}
