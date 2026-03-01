package contracts

// Environment represents the application environment.
type Environment string

const (
	EnvWeb     Environment = "web"
	EnvConsole Environment = "console"
	EnvTest    Environment = "test"
)

// ServiceProviderContract defines the interface for Service Providers.
// Service Providers are the central place for bootstrapping the application.
// They replicate Astra's ServiceProvider class with its lifecycle hooks.
//
// Lifecycle Order:
//  1. Register() — Bind services into the container (no other providers booted yet)
//  2. Boot() — All providers registered, safe to resolve other bindings
//  3. Ready() — Application is fully booted and ready to accept requests
//  4. Shutdown() — Application is shutting down, cleanup resources
type ServiceProviderContract interface {
	// Register binds services into the container.
	// Called before any other provider has booted.
	// Mirrors: register() in Astra providers
	Register() error

	// Boot is called after all providers have been registered.
	// Safe to resolve bindings from other providers here.
	// Mirrors: boot() in Astra providers
	Boot() error

	// Ready is called when the application is fully ready.
	// For web: after HTTP server starts listening.
	// For console: after command is ready to execute.
	// Mirrors: ready() in Astra providers
	Ready() error

	// Shutdown is called during graceful shutdown.
	// Clean up database connections, close Redis, flush queues, etc.
	// Mirrors: shutdown() in Astra providers
	Shutdown() error
}

// ApplicationContract defines the core application interface.
// It embeds the IoC container and manages the application lifecycle.
// This replicates Astra's Application class.
type ApplicationContract interface {
	// Embed the container — the application IS the container in Astra.
	ContainerContract

	// Environment returns the current application environment.
	Environment() Environment

	// IsReady returns true if the application has completed booting.
	IsReady() bool

	// Version returns the framework version string.
	Version() string

	// AppName returns the application name from config.
	AppName() string

	// AppRoot returns the root directory of the application.
	AppRoot() string

	// RegisterProvider registers a service provider with the application.
	// Mirrors: this.providers = [AppProvider, RouteProvider, ...]
	RegisterProvider(provider ServiceProviderContract)

	// RegisterProviders registers multiple service providers at once.
	RegisterProviders(providers []ServiceProviderContract)

	// Boot executes the full application lifecycle:
	//  1. Call Register() on all providers
	//  2. Call Boot() on all providers
	//  3. Mark application as booted
	Boot() error

	// Ready signals that the application is fully ready.
	// Calls Ready() on all providers.
	Ready() error

	// Shutdown gracefully shuts down the application.
	// Calls Shutdown() on all providers in reverse order.
	Shutdown() error

	// GetContainer provides direct access to the IoC container.
	GetContainer() ContainerContract
}
