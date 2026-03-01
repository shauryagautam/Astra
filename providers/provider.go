package providers

import "github.com/shaurya/astra/contracts"

// BaseProvider is a base struct for service providers.
// Embed it to inherit default no-op implementations of lifecycle hooks.
// Mirrors Astra's base ServiceProvider class.
type BaseProvider struct {
	App contracts.ApplicationContract
}

// NewBaseProvider creates a BaseProvider with the application reference.
func NewBaseProvider(app contracts.ApplicationContract) BaseProvider {
	return BaseProvider{App: app}
}

// Register is a no-op by default. Override in concrete providers.
func (p *BaseProvider) Register() error { return nil }

// Boot is a no-op by default. Override in concrete providers.
func (p *BaseProvider) Boot() error { return nil }

// Ready is a no-op by default. Override in concrete providers.
func (p *BaseProvider) Ready() error { return nil }

// Shutdown is a no-op by default. Override in concrete providers.
func (p *BaseProvider) Shutdown() error { return nil }

// Ensure BaseProvider implements ServiceProviderContract.
var _ contracts.ServiceProviderContract = (*BaseProvider)(nil)
