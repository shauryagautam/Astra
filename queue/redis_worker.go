package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astraframework/astra/events"
	"github.com/astraframework/astra/json"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var redisPendingRecoveryIdle = 30 * time.Second

const redisQueueProbeBlock = 50 * time.Millisecond

// WorkerMetrics exposes queue worker counters.
type WorkerMetrics struct {
	JobsProcessed int64 `json:"jobs_processed"`
	JobsFailed    int64 `json:"jobs_failed"`
	JobsRetried   int64 `json:"jobs_retried"`
	InFlight      int64 `json:"in_flight"`
}

// RedisWorker processes jobs from Redis Streams consumer groups.
type RedisWorker struct {
	client       redis.UniversalClient
	prefix       string
	queues       []string
	concurrency  int
	handlers     map[string]func() Job
	logger       *slog.Logger
	queue        *RedisQueue
	failed       *RedisFailedJobsStore
	events       *events.Emitter
	consumerName string

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup

	jobsProcessed atomic.Int64
	jobsFailed    atomic.Int64
	jobsRetried   atomic.Int64
	inFlight      atomic.Int64
	draining      atomic.Bool
}

// NewRedisWorker creates a new Redis worker.
func NewRedisWorker(client redis.UniversalClient, prefix string, queues []string, logger *slog.Logger) *RedisWorker {
	if len(queues) == 0 {
		queues = []string{defaultQueueName}
	}
	if logger == nil {
		logger = slog.Default()
	}
	prefix = normalizeQueuePrefix(prefix)
	queue := NewRedisQueue(client, prefix, nil).WithLogger(logger)
	return &RedisWorker{
		client:       client,
		prefix:       prefix,
		queues:       queues,
		concurrency:  1,
		handlers:     make(map[string]func() Job),
		logger:       logger,
		queue:        queue,
		failed:       NewRedisFailedJobsStore(client, prefix, queue),
		events:       events.DefaultEmitter,
		consumerName: "consumer-" + uuid.NewString(),
		stopCh:       make(chan struct{}),
	}
}

// WithEvents sets the event emitter for the worker.
func (w *RedisWorker) WithEvents(emitter *events.Emitter) *RedisWorker {
	w.events = emitter
	return w
}

// WithConcurrency sets the number of worker goroutines.
func (w *RedisWorker) WithConcurrency(n int) *RedisWorker {
	if n > 0 {
		w.concurrency = n
	}
	return w
}

// Register registers a named job factory.
func (w *RedisWorker) Register(name string, factory func() Job) {
	w.handlers[name] = factory
}

// Start begins polling Redis for new jobs.
func (w *RedisWorker) Start(ctx context.Context) error {
	if w.client == nil {
		return errNilRedisClient
	}
	for _, queueName := range w.queues {
		if err := ensureConsumerGroup(ctx, w.client, streamKey(w.prefix, queueName), consumerGroupName(w.prefix, queueName)); err != nil {
			return err
		}
		if err := w.recoverPending(ctx, queueName); err != nil {
			return err
		}
	}
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.run(ctx, i)
	}
	return nil
}

// Stop stops fetching new jobs and waits for in-flight jobs to finish.
// It respects the provided context for timeout protection.
func (w *RedisWorker) Stop(ctx context.Context) error {
	w.draining.Store(true)
	w.stopOnce.Do(func() { close(w.stopCh) })

	w.logger.Info("astra/queue: worker shutting down, draining in-flight jobs", "in_flight", w.inFlight.Load())

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		w.logger.Warn("astra/queue: worker shutdown timeout exceeded, forcing exit", "in_flight", w.inFlight.Load())
		return ctx.Err()
	case <-done:
		w.logger.Info("astra/queue: worker shutdown gracefully")
		return nil
	}
}

// Metrics returns the current worker counters.
func (w *RedisWorker) Metrics() WorkerMetrics {
	return WorkerMetrics{
		JobsProcessed: w.jobsProcessed.Load(),
		JobsFailed:    w.jobsFailed.Load(),
		JobsRetried:   w.jobsRetried.Load(),
		InFlight:      w.inFlight.Load(),
	}
}

func (w *RedisWorker) run(ctx context.Context, workerID int) {
	defer w.wg.Done()
	consumer := fmt.Sprintf("%s-%d", w.consumerName, workerID)
	queues := w.queuePollOrder(workerID)

	for {
		if w.draining.Load() {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
		}

		_, err := w.pollQueues(ctx, consumer, queues)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			w.logger.Error("astra/queue: worker poll failed", "consumer", consumer, "error", err)
			time.Sleep(250 * time.Millisecond)
		}
	}
}

func (w *RedisWorker) queuePollOrder(workerID int) []string {
	if len(w.queues) == 0 {
		return nil
	}
	start := workerID % len(w.queues)
	ordered := make([]string, 0, len(w.queues))
	for i := 0; i < len(w.queues); i++ {
		ordered = append(ordered, w.queues[(start+i)%len(w.queues)])
	}
	return ordered
}

func (w *RedisWorker) pollQueues(ctx context.Context, consumer string, queues []string) (bool, error) {
	for i, queueName := range queues {
		stream := streamKey(w.prefix, queueName)
		group := consumerGroupName(w.prefix, queueName)
		block := redisQueueProbeBlock
		if i == len(queues)-1 {
			block = 2 * time.Second
		}

		streams, err := w.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    block,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			return false, fmt.Errorf("queue %s: %w", queueName, err)
		}

		for _, streamBatch := range streams {
			for _, message := range streamBatch.Messages {
				w.processMessage(ctx, streamBatch.Stream, group, message)
			}
		}
		if len(streams) > 0 {
			return true, nil
		}
	}

	return false, nil
}

func (w *RedisWorker) processMessage(ctx context.Context, stream string, group string, message redis.XMessage) {
	envelope, err := decodeEnvelope(message)
	if err != nil {
		w.logger.Error("astra/queue: invalid job envelope", "stream", stream, "error", err)
		_ = w.client.XAck(ctx, stream, group, message.ID).Err()
		return
	}

	factory, ok := w.handlers[envelope.JobType]
	if !ok {
		w.logger.Error("astra/queue: missing job handler", "job_type", envelope.JobType)
		w.failJob(ctx, stream, group, message.ID, envelope, fmt.Errorf("astra/queue: missing job handler %s", envelope.JobType), nil)
		return
	}

	job := factory()
	if err := json.Unmarshal([]byte(envelope.Payload), job); err != nil {
		w.failJob(ctx, stream, group, message.ID, envelope, fmt.Errorf("astra/queue: %w", err), nil)
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, job.Timeout())
	defer cancel()

	w.inFlight.Add(1)
	defer w.inFlight.Add(-1)

	var runErr error
	var stack []byte
	start := time.Now()

	if w.events != nil {
		w.events.EmitPayload(ctx, "queue.job_started", map[string]any{
			"job_id":   envelope.ID,
			"job_type": envelope.JobType,
			"queue":    envelope.Queue,
		})
	}

	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				runErr = fmt.Errorf("astra/queue: panic: %v", recovered)
				stack = stackTrace()
			}
		}()
		runErr = job.Handle(jobCtx)
	}()

	duration := time.Since(start)

	if runErr == nil {
		w.jobsProcessed.Add(1)

		if w.events != nil {
			w.events.EmitPayload(ctx, "queue.job_completed", map[string]any{
				"job_id":      envelope.ID,
				"job_type":    envelope.JobType,
				"queue":       envelope.Queue,
				"duration_ms": duration.Milliseconds(),
			})
		}

		if err := w.client.XAck(ctx, stream, group, message.ID).Err(); err != nil {
			w.logger.Error("astra/queue: failed to ack job", "job_id", envelope.ID, "error", err)
		}
		return
	}

	if stack == nil {
		stack = stackTrace()
	}

	if w.events != nil {
		w.events.EmitPayload(ctx, "queue.job_failed", map[string]any{
			"job_id":      envelope.ID,
			"job_type":    envelope.JobType,
			"queue":       envelope.Queue,
			"error":       runErr.Error(),
			"duration_ms": duration.Milliseconds(),
		})
	}

	w.failJob(ctx, stream, group, message.ID, envelope, runErr, stack)
	job.OnFailure(ctx, runErr)
}

func (w *RedisWorker) failJob(ctx context.Context, stream string, group string, messageID string, envelope queueEnvelope, runErr error, stack []byte) {
	if err := w.client.XAck(ctx, stream, group, messageID).Err(); err != nil {
		w.logger.Error("astra/queue: failed to ack failed job", "job_id", envelope.ID, "error", err)
	}

	envelope.Attempts++
	if envelope.Attempts <= envelope.MaxRetries {
		w.jobsRetried.Add(1)
		if err := w.queue.enqueueEnvelope(ctx, envelope); err != nil {
			w.logger.Error("astra/queue: retry enqueue failed", "job_id", envelope.ID, "error", err)
		}
		return
	}

	w.jobsFailed.Add(1)
	if err := w.failed.Store(ctx, failureFromEnvelope(envelope, runErr, stack)); err != nil {
		w.logger.Error("astra/queue: failed storing failed job", "job_id", envelope.ID, "error", err)
	}
}

func (w *RedisWorker) recoverPending(ctx context.Context, queueName string) error {
	stream := streamKey(w.prefix, queueName)
	group := consumerGroupName(w.prefix, queueName)
	consumer := w.consumerName + "-recovery"
	start := "0-0"

	for {
		messages, nextStart, err := w.xautoclaim(ctx, stream, group, consumer, start)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return nil
		}
		for _, message := range messages {
			w.processMessage(ctx, stream, group, message)
		}
		if nextStart == start || nextStart == "0-0" {
			return nil
		}
		start = nextStart
	}
}

func (w *RedisWorker) xautoclaim(ctx context.Context, stream string, group string, consumer string, start string) ([]redis.XMessage, string, error) {
	result, nextStart, err := w.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  redisPendingRecoveryIdle,
		Start:    start,
		Count:    100,
	}).Result()
	if err == nil {
		return result, nextStart, nil
	}
	if strings.Contains(err.Error(), "unknown command") {
		return w.claimPendingFallback(ctx, stream, group, consumer)
	}
	if isRedisMissingGroup(err) {
		return nil, "0-0", nil
	}
	return nil, start, fmt.Errorf("astra/queue: %w", err)
}

func (w *RedisWorker) claimPendingFallback(ctx context.Context, stream string, group string, consumer string) ([]redis.XMessage, string, error) {
	pending, err := w.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		if isRedisMissingGroup(err) {
			return nil, "0-0", nil
		}
		return nil, "0-0", fmt.Errorf("astra/queue: %w", err)
	}
	var ids []string
	for _, item := range pending {
		if item.Idle >= redisPendingRecoveryIdle {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil, "0-0", nil
	}
	messages, err := w.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  redisPendingRecoveryIdle,
		Messages: ids,
	}).Result()
	if err != nil {
		return nil, "0-0", fmt.Errorf("astra/queue: %w", err)
	}
	return messages, "0-0", nil
}

// RedisFailedJobsStore persists failed jobs in Redis.
type RedisFailedJobsStore struct {
	client redis.UniversalClient
	prefix string
	queue  *RedisQueue
}

// NewRedisFailedJobsStore creates a failed job store.
func NewRedisFailedJobsStore(client redis.UniversalClient, prefix string, queue *RedisQueue) *RedisFailedJobsStore {
	return &RedisFailedJobsStore{
		client: client,
		prefix: normalizeQueuePrefix(prefix),
		queue:  queue,
	}
}

// Store persists a failed job entry.
func (s *RedisFailedJobsStore) Store(ctx context.Context, job FailedJob) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("astra/queue: %w", err)
	}
	return s.client.HSet(ctx, failedJobsKey(s.prefix), job.ID, body).Err()
}

// All returns all failed jobs.
func (s *RedisFailedJobsStore) All(ctx context.Context) ([]FailedJob, error) {
	items, err := s.client.HGetAll(ctx, failedJobsKey(s.prefix)).Result()
	if err != nil {
		return nil, fmt.Errorf("astra/queue: %w", err)
	}
	jobs := make([]FailedJob, 0, len(items))
	for _, raw := range items {
		var job FailedJob
		if err := json.Unmarshal([]byte(raw), &job); err != nil {
			return nil, fmt.Errorf("astra/queue: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// Find returns a failed job by ID.
func (s *RedisFailedJobsStore) Find(ctx context.Context, id string) (FailedJob, error) {
	raw, err := s.client.HGet(ctx, failedJobsKey(s.prefix), id).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return FailedJob{}, errFailedJobNotFound
		}
		return FailedJob{}, fmt.Errorf("astra/queue: %w", err)
	}
	var job FailedJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return FailedJob{}, fmt.Errorf("astra/queue: %w", err)
	}
	return job, nil
}

// Retry re-enqueues a failed job and removes it from the failed set.
func (s *RedisFailedJobsStore) Retry(ctx context.Context, id string) error {
	job, err := s.Find(ctx, id)
	if err != nil {
		return err
	}
	envelope := queueEnvelope{
		ID:         job.ID,
		Payload:    job.Payload,
		JobType:    job.JobType,
		Queue:      job.Queue,
		Attempts:   0,
		MaxRetries: job.MaxRetries,
		CreatedAt:  job.OriginalEnqueuedAt.UTC(),
	}
	if err := s.queue.enqueueEnvelope(ctx, envelope); err != nil {
		return err
	}
	return s.Delete(ctx, id)
}

// Delete removes a failed job entry.
func (s *RedisFailedJobsStore) Delete(ctx context.Context, id string) error {
	return s.client.HDel(ctx, failedJobsKey(s.prefix), id).Err()
}

// Purge removes all failed job entries.
func (s *RedisFailedJobsStore) Purge(ctx context.Context) error {
	return s.client.Del(ctx, failedJobsKey(s.prefix)).Err()
}
