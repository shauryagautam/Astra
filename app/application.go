package app

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/shaurya/astra/contracts"
)

// Application is the core of the Astra framework.
// It embeds the IoC Container and manages the complete application lifecycle.
// Replicates Astra's Application class.
type Application struct {
	container *Container

	mu        sync.RWMutex
	appName   string
	appRoot   string
	version   string
	env       contracts.Environment
	isReady   bool
	isBooted  bool
	providers []contracts.ServiceProviderContract
	logger    *log.Logger
}

// NewApplication creates a new Application with the given root directory.
func NewApplication(appRoot string) *Application {
	app := &Application{
		container: NewContainer(),
		appRoot:   appRoot,
		version:   "1.0.0",
		appName:   "Astra",
		env:       contracts.EnvWeb,
		providers: make([]contracts.ServiceProviderContract, 0),
		logger:    log.New(os.Stdout, "[astra] ", log.LstdFlags),
	}

	// Self-register the application in the container
	app.container.Singleton("Astra/Core/Application", func(c contracts.ContainerContract) (any, error) {
		return app, nil
	})
	app.container.Alias("app", "Astra/Core/Application")

	return app
}

// --- ContainerContract delegation ---
// Application delegates all container methods to its embedded container.

func (a *Application) Bind(namespace string, factory contracts.BindingFactory) {
	a.container.Bind(namespace, factory)
}

func (a *Application) Singleton(namespace string, factory contracts.BindingFactory) {
	a.container.Singleton(namespace, factory)
}

func (a *Application) Make(namespace string) (any, error) {
	return a.container.Make(namespace)
}

func (a *Application) MustMake(namespace string) any {
	return a.container.MustMake(namespace)
}

func (a *Application) Use(namespace string) any {
	return a.container.Use(namespace)
}

func (a *Application) HasBinding(namespace string) bool {
	return a.container.HasBinding(namespace)
}

func (a *Application) Alias(alias string, namespace string) {
	a.container.Alias(alias, namespace)
}

func (a *Application) Fake(namespace string, factory contracts.BindingFactory) {
	a.container.Fake(namespace, factory)
}

func (a *Application) Restore(namespace string) {
	a.container.Restore(namespace)
}

func (a *Application) Call(fn any, args ...any) ([]any, error) {
	return a.container.Call(fn, args...)
}

func (a *Application) WithBindings(namespaces []string, callback func(bindings map[string]any) error) error {
	return a.container.WithBindings(namespaces, callback)
}

func (a *Application) RegisterType(typ any, namespace string) {
	a.container.RegisterType(typ, namespace)
}

// --- ApplicationContract methods ---

// Environment returns the current application environment.
func (a *Application) Environment() contracts.Environment {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.env
}

// SetEnvironment sets the application environment.
func (a *Application) SetEnvironment(env contracts.Environment) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.env = env
}

// IsReady returns true if the application has completed booting.
func (a *Application) IsReady() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isReady
}

// Version returns the framework version string.
func (a *Application) Version() string {
	return a.version
}

// AppName returns the application name.
func (a *Application) AppName() string {
	return a.appName
}

// SetAppName sets the application name.
func (a *Application) SetAppName(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.appName = name
}

// AppRoot returns the root directory of the application.
func (a *Application) AppRoot() string {
	return a.appRoot
}

// RegisterProvider registers a single service provider.
func (a *Application) RegisterProvider(provider contracts.ServiceProviderContract) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers = append(a.providers, provider)
}

// RegisterProviders registers multiple service providers at once.
func (a *Application) RegisterProviders(providers []contracts.ServiceProviderContract) {
	for _, p := range providers {
		a.RegisterProvider(p)
	}
}

// Boot executes the full application lifecycle:
//  1. Call Register() on all providers
//  2. Call Boot() on all providers
//  3. Mark application as booted
func (a *Application) Boot() error {
	a.mu.Lock()
	if a.isBooted {
		a.mu.Unlock()
		return nil
	}
	providers := make([]contracts.ServiceProviderContract, len(a.providers))
	copy(providers, a.providers)
	a.mu.Unlock()

	// Phase 1: Register all providers
	a.logger.Println("⚡ Registering service providers...")
	for _, provider := range providers {
		if err := provider.Register(); err != nil {
			return fmt.Errorf("provider registration failed: %w", err)
		}
	}

	// Phase 2: Boot all providers
	a.logger.Println("⚡ Booting service providers...")
	for _, provider := range providers {
		if err := provider.Boot(); err != nil {
			return fmt.Errorf("provider boot failed: %w", err)
		}
	}

	a.mu.Lock()
	a.isBooted = true
	a.mu.Unlock()

	a.logger.Println("✅ Application booted successfully")
	return nil
}

// Ready signals that the application is fully ready.
func (a *Application) Ready() error {
	a.mu.Lock()
	providers := make([]contracts.ServiceProviderContract, len(a.providers))
	copy(providers, a.providers)
	a.mu.Unlock()

	for _, provider := range providers {
		if err := provider.Ready(); err != nil {
			return fmt.Errorf("provider ready failed: %w", err)
		}
	}

	a.mu.Lock()
	a.isReady = true
	a.mu.Unlock()

	return nil
}

// Shutdown gracefully shuts down the application.
func (a *Application) Shutdown() error {
	a.logger.Println("🔄 Shutting down application...")

	a.mu.Lock()
	providers := make([]contracts.ServiceProviderContract, len(a.providers))
	copy(providers, a.providers)
	a.isReady = false
	a.isBooted = false
	a.mu.Unlock()

	for i := len(providers) - 1; i >= 0; i-- {
		if err := providers[i].Shutdown(); err != nil {
			a.logger.Printf("⚠️  Provider shutdown error: %v", err)
		}
	}

	a.logger.Println("✅ Application shut down")
	return nil
}

// GetContainer provides direct access to the IoC container.
func (a *Application) GetContainer() contracts.ContainerContract {
	return a.container
}

// Logger returns the application logger.
func (a *Application) Logger() *log.Logger {
	return a.logger
}

// Ensure Application implements ApplicationContract at compile time.
var _ contracts.ApplicationContract = (*Application)(nil)
