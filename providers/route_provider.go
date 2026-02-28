package providers

import (
	adonisHttp "github.com/shaurya/adonis/app/Http"
	"github.com/shaurya/adonis/contracts"
)

// RouteProvider registers the Router and HTTP Server into the container.
// Mirrors AdonisJS's RouteProvider.
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
	p.App.Singleton("Adonis/Core/Route", func(c contracts.ContainerContract) (any, error) {
		return adonisHttp.NewRouter(), nil
	})
	p.App.Alias("Route", "Adonis/Core/Route")

	// Register the HTTP Server as a singleton
	p.App.Singleton("Adonis/Core/Server", func(c contracts.ContainerContract) (any, error) {
		server := adonisHttp.NewServer()
		// Wire the router into the server
		router := c.Use("Route").(contracts.RouterContract)
		server.SetRouter(router)
		return server, nil
	})
	p.App.Alias("Server", "Adonis/Core/Server")

	return nil
}

// Boot registers default middleware on the server.
func (p *RouteProvider) Boot() error {
	server := p.App.Use("Server").(*adonisHttp.Server)

	// Register default global middleware stack
	server.Use(
		adonisHttp.RecoveryMiddleware(),
		adonisHttp.LoggerMiddleware(),
		adonisHttp.SecureHeadersMiddleware(),
	)

	return nil
}
