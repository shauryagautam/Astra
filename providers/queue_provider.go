package providers

import (
	queue "github.com/shaurya/astra/app/Queue"
	"github.com/shaurya/astra/contracts"
)

// QueueProvider registers the Queue manager and Registry into the container.
// Mirrors Astra's Queue provider.
type QueueProvider struct {
	BaseProvider
}

// NewQueueProvider creates a new QueueProvider.
func NewQueueProvider(app contracts.ApplicationContract) *QueueProvider {
	return &QueueProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds the Queue manager and Registry as singletons.
func (p *QueueProvider) Register() error {
	// Register the Job Registry
	p.App.Singleton("Astra/Core/JobRegistry", func(c contracts.ContainerContract) (any, error) {
		return queue.NewRegistry(), nil
	})

	// Register the Queue Manager
	p.App.Singleton("Astra/Core/Queue", func(c contracts.ContainerContract) (any, error) {
		// Use the Redis manager to get the default connection
		redisManager, err := c.Make("Redis")
		if err != nil {
			return nil, err
		}

		manager := redisManager.(contracts.RedisContract)
		conn := manager.Connection("default")

		registry := c.Use("Astra/Core/JobRegistry").(contracts.JobRegistry)
		return queue.NewRedisQueue(conn, registry), nil
	})

	p.App.Alias("Queue", "Astra/Core/Queue")
	p.App.Alias("JobRegistry", "Astra/Core/JobRegistry")

	return nil
}

// Boot is a no-op for the QueueProvider.
func (p *QueueProvider) Boot() error {
	return nil
}
