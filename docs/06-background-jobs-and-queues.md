# Background jobs and queues

> Source files reviewed for this document: `pkg/queue/job.go`,
> `job_generic.go`, `redis_queue.go`, `redis_dispatcher.go`,
> `redis_worker.go`, `failed_jobs.go`, `scheduler.go`, and queue tests.

## Job contract

The queue package defines a `Job` interface. A job handles a context and can
provide queue name, timeout, retry count, and failure behavior:

```go
type EmailJob struct {
    queue.BaseJob
    To string `json:"to"`
}

func (j *EmailJob) Handle(ctx context.Context) error {
    return sendEmail(ctx, j.To)
}
```

`BaseJob` supplies defaults. `GenericJob[T]` is useful when the payload is a
typed value:

```go
job := queue.NewJob("send-email", SendEmail{To: "user@example.com"}).
    WithQueue("mail").
    WithMaxRetries(5).
    WithTimeout(30 * time.Second)
```

## Redis queue

`RedisQueue` stores jobs in Redis Streams:

```go
q := queue.NewRedisQueue(redisClient, "app:", nil)
if err := q.Enqueue(ctx, job); err != nil {
    return err
}
if err := q.EnqueueIn(ctx, job, time.Minute); err != nil {
    return err
}
```

`EnqueueAt`, `Size`, `Purge`, `Start`, and `Stop` are also available. Delayed
jobs are promoted by the queue/scheduler machinery.

## Dispatching

`RedisDispatcher` provides `Dispatch`, `DispatchUnique`, `DispatchIn`, and
`DispatchAt`. `DispatchUnique` uses a TTL to suppress duplicate dispatches.
The `Dispatcher` and `StreamsDriver` names are aliases for compatibility.

## Workers

Construct a worker with queues and concurrency:

```go
worker := queue.NewRedisWorker(
    redisClient,
    "app:",
    []string{"default", "mail"},
    logger,
).WithConcurrency(8)

worker.Register("send-email", func() queue.Job {
    return new(EmailJob)
})

if err := worker.Start(ctx); err != nil {
    return err
}
defer worker.Stop(shutdownCtx)
```

Workers use Redis Streams consumer groups, recover pending work, process jobs
with per-job timeouts, and expose `WorkerMetrics`. `Stop` stops fetching and
waits for in-flight jobs until its context expires.

The handler registry is application state. Register all job types before
starting workers and protect registration if your application mutates it
concurrently.

## Failure handling

Failed jobs are stored by `RedisFailedJobsStore`. `FailedJobManager` and worker
methods support listing, retrying, deleting, and purging failed work. A job's
`OnFailure` method is called by the worker after processing failure.

Retries should be reserved for transient errors. Make handlers idempotent:
Redis delivery and process termination can produce duplicate execution.

## Scheduling

`Scheduler.Register` accepts a named cron job and a seconds-aware cron spec:

```go
scheduler := queue.NewScheduler(redisClient, "app:", q)
_, err := scheduler.Register("refresh-cache", "0 */5 * * * *", func() {
    refreshCache()
})
```

When Redis is configured, named jobs use a distributed lock to avoid duplicate
execution across service instances. `Start` begins cron processing and delayed
job promotion; `Stop` waits for running cron jobs or its context.

`Schedule` is an anonymous compatibility API without distributed locking.

## Tracing and events

Workers can receive an `event.Emitter` and dashboard tracer. Redis queue
envelopes carry W3C trace context, allowing a worker span to link back to the
enqueueing request when OpenTelemetry is configured.

## Operational guidance

- Give workers a bounded shutdown context.
- Monitor `JobsFailed`, `JobsRetried`, and `InFlight`.
- Set explicit job timeouts.
- Keep payloads small and JSON-compatible.
- Make external side effects idempotent.
- Use separate queue names for workloads with different latency or retry needs.
