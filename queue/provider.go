package queue

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/core"
	"github.com/redis/go-redis/v9"
)

// QueueProvider manages the lifecycle of the background job workers.
type QueueProvider struct {
	core.BaseProvider
}

// Register assembles the Queue service into the container.
func (p *QueueProvider) Register(a *core.App) error {
	redisClient, ok := a.Get("redis").(redis.UniversalClient)
	if !ok {
		// If redis is not available, we might still want to register a dispatcher
		// that fails or uses a different driver. For now, we assume redis is required.
		return nil
	}

	queueConfig := a.Config.Queue
	worker := NewWorker(
		redisClient,
		queueConfig.Prefix,
		queueConfig.Queues,
		queueConfig.Concurrency,
		a.Logger,
	)

	a.Register("queue.worker", worker)
	return nil
}

// Boot starts the background worker.
func (p *QueueProvider) Boot(a *core.App) error {
	svc := a.Get("queue.worker")
	if svc == nil {
		return nil
	}

	worker, ok := svc.(*Worker)
	if !ok {
		return fmt.Errorf("queue: registered worker is not of type *queue.Worker")
	}

	a.Logger.Info("starting queue worker...")
	return worker.Start(context.Background())
}

// Shutdown gracefully stops the background worker.
func (p *QueueProvider) Shutdown(ctx context.Context, a *core.App) error {
	svc := a.Get("queue.worker")
	if svc == nil {
		return nil
	}

	worker, ok := svc.(*Worker)
	if !ok {
		return nil
	}

	a.Logger.Info("stopping queue worker...")
	return worker.Stop(ctx)
}
