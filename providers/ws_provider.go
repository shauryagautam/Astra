package providers

import (
	ws "github.com/shaurya/astra/app/Ws"
	"github.com/shaurya/astra/contracts"
)

// WsProvider registers the WebSocket hub into the container.
// Mirrors Astra's @astra/websocket provider.
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
	p.App.Singleton("Astra/Core/Ws", func(c contracts.ContainerContract) (any, error) {
		hub := ws.NewHub()
		// Start the hub loop in a goroutine
		go hub.Start()
		return hub, nil
	})

	p.App.Alias("Ws", "Astra/Core/Ws")

	return nil
}

// Boot is a no-op for the WsProvider.
func (p *WsProvider) Boot() error {
	return nil
}
