package providers

import (
	astraHttp "github.com/shaurya/astra/app/Http"
	"github.com/shaurya/astra/contracts"
)

// RouteProvider registers the Router and HTTP Server into the container.
// Mirrors Astra's RouteProvider.
type RouteProvider struct {
	BaseProvider
}

// NewRouteProvider creates a new RouteProvider.
func NewRouteProvider(app contracts.ApplicationContract) *RouteProvider {
	return &RouteProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds the Router and Server into the container.
func (p *RouteProvider) Register() error {
	// Register the Router as a singleton
	p.App.Singleton("Astra/Core/Route", func(c contracts.ContainerContract) (any, error) {
		return astraHttp.NewRouter(), nil
	})
	p.App.Alias("Route", "Astra/Core/Route")

	// Register the HTTP Server as a singleton
	p.App.Singleton("Astra/Core/Server", func(c contracts.ContainerContract) (any, error) {
		server := astraHttp.NewServer()
		// Wire the router into the server
		router := c.Use("Route").(contracts.RouterContract)
		server.SetRouter(router)
		return server, nil
	})
	p.App.Alias("Server", "Astra/Core/Server")

	return nil
}

// Boot registers default middleware on the server.
func (p *RouteProvider) Boot() error {
	server := p.App.Use("Server").(*astraHttp.Server)

	// Register default global middleware stack
	server.Use(
		astraHttp.RecoveryMiddleware(),
		astraHttp.LoggerMiddleware(),
		astraHttp.SecureHeadersMiddleware(),
	)

	return nil
}
