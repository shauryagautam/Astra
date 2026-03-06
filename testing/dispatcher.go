package testing

import (
	"context"
	"time"

	"github.com/astraframework/astra/queue"
)

// SyncDispatcher executes jobs immmediately in the same goroutine instead of queueing.
type SyncDispatcher struct{}

// NewSyncDispatcher creates a new synchronous dispatcher.
func NewSyncDispatcher() *SyncDispatcher {
	return &SyncDispatcher{}
}

// Dispatch executes the job immediately.
func (d *SyncDispatcher) Dispatch(ctx context.Context, job queue.Job, name string) error {
	return job.Handle()
}

// DispatchIn executes the job immediately (ignoring delay) for testing purposes.
func (d *SyncDispatcher) DispatchIn(ctx context.Context, job queue.Job, name string, delay time.Duration) error {
	return job.Handle()
}

// DispatchAt executes the job immediately (ignoring scheduled time) for testing purposes.
func (d *SyncDispatcher) DispatchAt(ctx context.Context, job queue.Job, name string, at time.Time) error {
	return job.Handle()
}
