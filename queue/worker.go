package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

// Worker polls a Redis queue and processes jobs.
type Worker struct {
	client      redis.UniversalClient
	prefix      string
	queues      []string
	concurrency int
	handlers    map[string]func() Job
	logger      *slog.Logger
	stop        chan struct{}
	wg          sync.WaitGroup
}

// NewWorker creates a new Worker.
func NewWorker(client redis.UniversalClient, prefix string, queues []string, concurrency int, logger *slog.Logger) *Worker {
	if len(queues) == 0 {
		queues = []string{"default"}
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	return &Worker{
		client:      client,
		prefix:      prefix,
		queues:      queues,
		concurrency: concurrency,
		handlers:    make(map[string]func() Job),
		logger:      logger,
		stop:        make(chan struct{}),
	}
}

// Register registers a job factory by name.
func (w *Worker) Register(name string, factory func() Job) {
	w.handlers[name] = factory
}

// Start begins polling for jobs.
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("starting queue worker", "concurrency", w.concurrency, "queues", w.queues)

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.work(ctx, i)
	}

	return nil
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop(ctx context.Context) error {
	w.logger.Info("stopping queue worker...")
	close(w.stop)
	w.wg.Wait()
	w.logger.Info("queue worker stopped")
	return nil
}

func (w *Worker) work(ctx context.Context, id int) {
	defer w.wg.Done()

	keys := make([]string, len(w.queues))
	for i, q := range w.queues {
		keys[i] = w.prefix + q
	}

	for {
		select {
		case <-w.stop:
			return
		case <-ctx.Done():
			return
		default:
			// BLPOP blocks until a job is available or timeout
			res, err := w.client.BLPop(ctx, 2*time.Second, keys...).Result()
			if err != nil {
				if err != redis.Nil {
					w.logger.Error("redis error while polling", "error", err)
					time.Sleep(1 * time.Second)
				}
				continue
			}

			if len(res) == 2 {
				queueName := res[0]
				payloadData := res[1]
				w.processPayload(ctx, queueName, payloadData)
			}
		}
	}
}

func (w *Worker) processPayload(ctx context.Context, queueName string, data string) {
	var p payload
	if err := sonic.Unmarshal([]byte(data), &p); err != nil {
		w.logger.Error("failed to unmarshal job payload", "error", err)
		return
	}

	factory, ok := w.handlers[p.Name]
	if !ok {
		w.logger.Error("unknown job name", "name", p.Name)
		w.handleFailure(ctx, p, fmt.Errorf("unknown job name: %s", p.Name))
		return
	}

	job := factory()
	if err := sonic.Unmarshal(p.Data, &job); err != nil {
		w.logger.Error("failed to unmarshal job data", "error", err)
		w.handleFailure(ctx, p, err)
		return
	}

	p.Attempts++
	start := time.Now()

	w.logger.Info("processing job", "name", p.Name, "attempt", p.Attempts)

	err := job.Handle()

	if err != nil {
		w.logger.Error("job failed", "name", p.Name, "error", err, "duration", time.Since(start))
		w.handleFailure(ctx, p, err)
	} else {
		w.logger.Info("job completed successfully", "name", p.Name, "duration", time.Since(start))
	}
}

func (w *Worker) handleFailure(ctx context.Context, p payload, jobErr error) {
	if p.Attempts >= p.Retries {
		// Max retries reached, move to failed jobs
		w.logger.Error("job failed permanently", "name", p.Name)
		failedData, _ := sonic.Marshal(p)
		w.client.LPush(ctx, w.prefix+"failed_jobs", failedData)
		return
	}

	// Calculate exponential backoff
	backoff := time.Duration(1<<p.Attempts) * 5 * time.Second

	// Delay retry
	pData, _ := sonic.Marshal(p)
	delayedKey := w.prefix + "delayed:" + p.Name // Approximation, we don't have the original queue
	w.client.ZAdd(ctx, delayedKey, redis.Z{
		Score:  float64(time.Now().Add(backoff).Unix()),
		Member: pData,
	})
}
