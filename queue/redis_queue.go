package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/astraframework/astra/cache"
	"github.com/astraframework/astra/json"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

var (
	errNilRedisClient    = errors.New("astra/queue: redis client is nil")
	errNilJob            = errors.New("astra/queue: job is nil")
	errFailedJobNotFound = errors.New("astra/queue: failed job not found")
)

// FailedJob represents a job that exhausted all retries.
type FailedJob struct {
	ID                 string    `json:"id"`
	JobType            string    `json:"job_type"`
	Queue              string    `json:"queue"`
	Payload            string    `json:"payload"`
	Error              string    `json:"error"`
	StackTrace         string    `json:"stack_trace,omitempty"`
	FailedAt           time.Time `json:"failed_at"`
	Attempts           int       `json:"attempts"`
	MaxRetries         int       `json:"max_retries"`
	OriginalEnqueuedAt time.Time `json:"original_enqueued_at"`
}

type queueEnvelope struct {
	ID         string    `json:"id"`
	Payload    string    `json:"payload"`
	JobType    string    `json:"job_type"`
	Queue      string    `json:"queue"`
	Attempts   int       `json:"attempts"`
	MaxRetries int       `json:"max_retries"`
	CreatedAt  time.Time `json:"created_at"`
	TraceID    string    `json:"trace_id,omitempty"`
}

type delayedEnvelope struct {
	RunAt time.Time     `json:"run_at"`
	Job   queueEnvelope `json:"job"`
}

// RedisQueue is a Redis Streams-backed persistent queue.
type RedisQueue struct {
	client           redis.UniversalClient
	locker           cache.Locker
	logger           *slog.Logger
	prefix           string
	delayedKey       string
	promoterInterval time.Duration
	promoterStop     chan struct{}
	promoterDone     sync.WaitGroup
}

// NewRedisQueue creates a new Redis-backed queue.
func NewRedisQueue(client redis.UniversalClient, prefix string, locker cache.Locker) *RedisQueue {
	return &RedisQueue{
		client:           client,
		locker:           locker,
		logger:           slog.Default(),
		prefix:           normalizeQueuePrefix(prefix),
		delayedKey:       delayedQueueKey(normalizeQueuePrefix(prefix)),
		promoterInterval: defaultPollInterval,
		promoterStop:     make(chan struct{}),
	}
}

// WithLogger sets the logger used by the queue.
func (q *RedisQueue) WithLogger(logger *slog.Logger) *RedisQueue {
	if logger != nil {
		q.logger = logger
	}
	return q
}

// Enqueue stores a job for immediate execution.
func (q *RedisQueue) Enqueue(ctx context.Context, job Job) error {
	return q.enqueue(ctx, jobTypeName(job), job, 0)
}

// EnqueueIn stores a job for later execution.
func (q *RedisQueue) EnqueueIn(ctx context.Context, job Job, delay time.Duration) error {
	return q.EnqueueAt(ctx, job, time.Now().Add(delay))
}

// EnqueueAt stores a job for execution at a specific time.
func (q *RedisQueue) EnqueueAt(ctx context.Context, job Job, at time.Time) error {
	envelope, err := newQueueEnvelope(ctx, jobTypeName(job), job, 0)
	if err != nil {
		return err
	}
	body, err := json.Marshal(delayedEnvelope{RunAt: at.UTC(), Job: envelope})
	if err != nil {
		return fmt.Errorf("astra/queue: %w", err)
	}
	return q.client.ZAdd(ctx, q.delayedKey, redis.Z{
		Score:  float64(at.Unix()),
		Member: body,
	}).Err()
}

// Size reports the number of ready jobs in a stream.
func (q *RedisQueue) Size(ctx context.Context, queue string) (int64, error) {
	return q.client.XLen(ctx, streamKey(q.prefix, queue)).Result()
}

// Purge removes all pending jobs for the provided queue.
func (q *RedisQueue) Purge(ctx context.Context, queue string) error {
	stream := streamKey(q.prefix, queue)
	group := consumerGroupName(q.prefix, queue)

	if err := q.client.Del(ctx, stream).Err(); err != nil {
		return fmt.Errorf("astra/queue: %w", err)
	}
	if err := q.client.XGroupDestroy(ctx, stream, group).Err(); err != nil && !isRedisMissingGroup(err) {
		return fmt.Errorf("astra/queue: %w", err)
	}
	return nil
}

// Start starts the delayed job promoter.
func (q *RedisQueue) Start(ctx context.Context) error {
	if q.client == nil {
		return errNilRedisClient
	}
	q.promoterDone.Add(1)
	go q.promoteLoop(ctx)
	return nil
}

// Stop stops the delayed job promoter.
func (q *RedisQueue) Stop(ctx context.Context) error {
	select {
	case <-q.promoterStop:
	default:
		close(q.promoterStop)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		q.promoterDone.Wait()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (q *RedisQueue) enqueue(ctx context.Context, jobType string, job Job, attempts int) error {
	envelope, err := newQueueEnvelope(ctx, jobType, job, attempts)
	if err != nil {
		return err
	}
	return q.enqueueEnvelope(ctx, envelope)
}

func (q *RedisQueue) enqueueEnvelope(ctx context.Context, envelope queueEnvelope) error {
	if q.client == nil {
		return errNilRedisClient
	}
	if err := ensureConsumerGroup(ctx, q.client, streamKey(q.prefix, envelope.Queue), consumerGroupName(q.prefix, envelope.Queue)); err != nil {
		return err
	}
	values := map[string]any{
		"id":          envelope.ID,
		"payload":     envelope.Payload,
		"job_type":    envelope.JobType,
		"attempts":    envelope.Attempts,
		"max_retries": envelope.MaxRetries,
		"created_at":  envelope.CreatedAt.Format(time.RFC3339),
		"queue":       envelope.Queue,
	}
	if err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(q.prefix, envelope.Queue),
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("astra/queue: %w", err)
	}
	return nil
}

func (q *RedisQueue) promoteLoop(ctx context.Context) {
	defer q.promoterDone.Done()

	ticker := time.NewTicker(q.promoterInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.promoterStop:
			return
		case <-ticker.C:
			if err := q.promoteReady(ctx); err != nil {
				q.logger.Error("astra: delayed queue promotion failed", "error", err)
			}
		}
	}
}

func (q *RedisQueue) promoteReady(ctx context.Context) error {
	if q.client == nil {
		return errNilRedisClient
	}

	var lock cache.Lock
	var err error
	if q.locker != nil {
		lock, err = q.locker.Acquire(ctx, "queue:delayed:promoter", 2*time.Second)
		if err != nil {
			if errors.Is(err, cache.ErrLockNotAcquired) {
				return nil
			}
			return fmt.Errorf("astra/queue: %w", err)
		}
		defer func() {
			_ = lock.Release(context.Background())
		}()
	}

	now := time.Now().Unix()
	items, err := q.client.ZRangeByScore(ctx, q.delayedKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()
	if err != nil {
		return fmt.Errorf("astra/queue: %w", err)
	}
	for _, item := range items {
		var delayed delayedEnvelope
		if err := json.Unmarshal([]byte(item), &delayed); err != nil {
			return fmt.Errorf("astra/queue: %w", err)
		}
		if err := q.enqueueEnvelope(ctx, delayed.Job); err != nil {
			return err
		}
		if err := q.client.ZRem(ctx, q.delayedKey, item).Err(); err != nil {
			return fmt.Errorf("astra/queue: %w", err)
		}
	}
	return nil
}

func newQueueEnvelope(ctx context.Context, jobType string, job Job, attempts int) (queueEnvelope, error) {
	if job == nil {
		return queueEnvelope{}, errNilJob
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return queueEnvelope{}, fmt.Errorf("astra/queue: %w", err)
	}
	maxRetries := job.MaxRetries()
	if maxRetries < 0 {
		maxRetries = 0
	}
	queueName := strings.TrimSpace(job.Queue())
	if queueName == "" {
		queueName = defaultQueueName
	}

	traceID := ""
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.HasTraceID() {
		traceID = spanCtx.TraceID().String()
	}

	return queueEnvelope{
		ID:         uuid.NewString(),
		Payload:    string(payload),
		JobType:    jobType,
		Queue:      queueName,
		Attempts:   attempts,
		MaxRetries: maxRetries,
		CreatedAt:  time.Now().UTC(),
		TraceID:    traceID,
	}, nil
}

func jobTypeName(job Job) string {
	if job == nil {
		return ""
	}
	typ := reflect.TypeOf(job)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Name()
}

func decodeEnvelope(message redis.XMessage) (queueEnvelope, error) {
	attempts, err := toInt(message.Values["attempts"])
	if err != nil {
		return queueEnvelope{}, err
	}
	maxRetries, err := toInt(message.Values["max_retries"])
	if err != nil {
		return queueEnvelope{}, err
	}
	createdAt, err := time.Parse(time.RFC3339, toString(message.Values["created_at"]))
	if err != nil {
		return queueEnvelope{}, fmt.Errorf("astra/queue: %w", err)
	}
	return queueEnvelope{
		ID:         toString(message.Values["id"]),
		Payload:    toString(message.Values["payload"]),
		JobType:    toString(message.Values["job_type"]),
		Queue:      toString(message.Values["queue"]),
		Attempts:   attempts,
		MaxRetries: maxRetries,
		CreatedAt:  createdAt,
	}, nil
}

func ensureConsumerGroup(ctx context.Context, client redis.UniversalClient, stream string, group string) error {
	if client == nil {
		return errNilRedisClient
	}
	if err := client.XGroupCreateMkStream(ctx, stream, group, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("astra/queue: %w", err)
	}
	return nil
}

func normalizeQueuePrefix(prefix string) string {
	trimmed := strings.Trim(strings.TrimSpace(prefix), ":")
	if trimmed == "" || trimmed == "queue" {
		return defaultQueuePrefix
	}
	if strings.HasSuffix(trimmed, ":queue") {
		return strings.TrimSuffix(trimmed, ":queue")
	}
	if strings.HasSuffix(trimmed, "queue") {
		// handle "myqueue" -> "my" (maybe? but let's stick to what was there but safer)
		return strings.TrimSuffix(strings.TrimSuffix(trimmed, "queue"), ":")
	}
	return trimmed
}

func streamKey(prefix string, queue string) string {
	return prefix + ":queue:" + queue
}

func delayedQueueKey(prefix string) string {
	return prefix + ":queue:delayed"
}

func consumerGroupName(prefix string, queue string) string {
	return prefix + ":workers:" + queue
}

func failedJobsKey(prefix string) string {
	return prefix + ":failed_jobs"
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func toInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case string:
		var parsed int
		_, err := fmt.Sscanf(v, "%d", &parsed)
		return parsed, err
	default:
		var parsed int
		_, err := fmt.Sscanf(fmt.Sprint(v), "%d", &parsed)
		return parsed, err
	}
}

func failureFromEnvelope(envelope queueEnvelope, err error, stack []byte) FailedJob {
	return FailedJob{
		ID:                 envelope.ID,
		JobType:            envelope.JobType,
		Queue:              envelope.Queue,
		Payload:            envelope.Payload,
		Error:              err.Error(),
		StackTrace:         string(stack),
		FailedAt:           time.Now().UTC(),
		Attempts:           envelope.Attempts,
		MaxRetries:         envelope.MaxRetries,
		OriginalEnqueuedAt: envelope.CreatedAt.UTC(),
	}
}

func stackTrace() []byte {
	return debug.Stack()
}

func isRedisMissingGroup(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "NOGROUP") || strings.Contains(err.Error(), "ERR no such key"))
}
