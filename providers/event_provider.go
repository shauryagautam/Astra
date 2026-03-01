package providers

import (
	events "github.com/shaurya/astra/app/Events"
	"github.com/shaurya/astra/contracts"
)

// EventProvider registers the Event dispatcher into the container.
// Mirrors Astra's Event provider.
type EventProvider struct {
	BaseProvider
}

// NewEventProvider creates a new EventProvider.
func NewEventProvider(app contracts.ApplicationContract) *EventProvider {
	return &EventProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds the Event dispatcher as a singleton.
func (p *EventProvider) Register() error {
	p.App.Singleton("Astra/Core/Event", func(c contracts.ContainerContract) (any, error) {
		return events.NewDispatcher(), nil
	})
	p.App.Alias("Event", "Astra/Core/Event")
	return nil
}

// Boot is a no-op for the EventProvider.
func (p *EventProvider) Boot() error {
	return nil
}
