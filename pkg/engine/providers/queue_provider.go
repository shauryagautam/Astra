package providers

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/telemetry"
	"github.com/shauryagautam/Astra/pkg/queue"
)

type QueueProvider struct {
	engine.BaseProvider
	redisClient redis.UniversalClient
	dash        *telemetry.Dashboard
	worker      *queue.Worker
}

func NewQueueProvider(r redis.UniversalClient, d *telemetry.Dashboard) *QueueProvider {
	return &QueueProvider{
		redisClient: r,
		dash:        d,
	}
}

func (p *QueueProvider) Name() string { return "queue" }

func (p *QueueProvider) Register(a *engine.App) error {
	if p.redisClient == nil {
		return fmt.Errorf("queue: redis client is required")
	}

	cfg := a.Config().Queue
	p.worker = queue.NewWorker(
		p.redisClient,
		cfg.Prefix,
		cfg.Queues,
		cfg.Concurrency,
		a.Logger(),
	)

	// Integration: Hook into telemetry if available
	if p.dash != nil {
		p.worker.WithDashboard(p.dash)
	}

	return nil
}


func (p *QueueProvider) Boot(a *engine.App) error {
	if p.worker != nil {
		// Start the worker in the background using the app context
		go func() {
			if err := p.worker.Start(a.BaseContext()); err != nil {
				a.Logger().Error("queue: worker failed to start", "error", err)
			}
		}()
	}
	return nil
}

func (p *QueueProvider) Shutdown(ctx context.Context, a *engine.App) error {
	if p.worker != nil {
		return p.worker.Stop(ctx)
	}
	return nil
}
