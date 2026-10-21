// Package queue provides Redis-backed job dispatch and worker primitives.
//
// A dispatcher accepts jobs, while a worker is configured with explicit job
// handlers and processes them with context-aware retries and shutdown. The
// package has no process-wide job registry.
package queue
