package providers

import (
	events "github.com/shaurya/adonis/app/Events"
	"github.com/shaurya/adonis/contracts"
)

// EventProvider registers the Event dispatcher into the container.
// Mirrors AdonisJS's Event provider.
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
	p.App.Singleton("Adonis/Core/Event", func(c contracts.ContainerContract) (any, error) {
		return events.NewDispatcher(), nil
	})
	p.App.Alias("Event", "Adonis/Core/Event")
	return nil
}

// Boot is a no-op for the EventProvider.
func (p *EventProvider) Boot() error {
	return nil
}
