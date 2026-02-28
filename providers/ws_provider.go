package providers

import (
	ws "github.com/shaurya/adonis/app/Ws"
	"github.com/shaurya/adonis/contracts"
)

// WsProvider registers the WebSocket hub into the container.
// Mirrors AdonisJS's @adonisjs/websocket provider.
type WsProvider struct {
	BaseProvider
}

// NewWsProvider creates a new WsProvider.
func NewWsProvider(app contracts.ApplicationContract) *WsProvider {
	return &WsProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds the Ws Hub as a singleton.
func (p *WsProvider) Register() error {
	p.App.Singleton("Adonis/Core/Ws", func(c contracts.ContainerContract) (any, error) {
		hub := ws.NewHub()
		// Start the hub loop in a goroutine
		go hub.Start()
		return hub, nil
	})

	p.App.Alias("Ws", "Adonis/Core/Ws")

	return nil
}

// Boot is a no-op for the WsProvider.
func (p *WsProvider) Boot() error {
	return nil
}
