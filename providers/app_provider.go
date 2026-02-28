package providers

import (
	"log"
	"os"

	"github.com/shaurya/adonis/contracts"
)

// AppProvider registers core application services into the container.
// This is the first provider registered, wiring up Logger, Env, etc.
// Mirrors AdonisJS's AppProvider.
type AppProvider struct {
	BaseProvider
}

// NewAppProvider creates a new AppProvider.
func NewAppProvider(app contracts.ApplicationContract) *AppProvider {
	return &AppProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds core services into the container.
func (p *AppProvider) Register() error {
	// Register the Logger
	p.App.Singleton("Adonis/Core/Logger", func(c contracts.ContainerContract) (any, error) {
		return log.New(os.Stdout, "[adonis] ", log.LstdFlags), nil
	})
	p.App.Alias("Logger", "Adonis/Core/Logger")

	// Register the Env manager
	p.App.Singleton("Adonis/Core/Env", func(c contracts.ContainerContract) (any, error) {
		return NewEnvManager(), nil
	})
	p.App.Alias("Env", "Adonis/Core/Env")

	return nil
}

// EnvManager handles environment variable access.
// Mirrors AdonisJS's Env module.
type EnvManager struct {
	overrides map[string]string
}

// NewEnvManager creates a new EnvManager.
func NewEnvManager() *EnvManager {
	return &EnvManager{
		overrides: make(map[string]string),
	}
}

// Get returns environment variable value or a default.
// Mirrors: Env.get('KEY', 'default')
func (e *EnvManager) Get(key string, defaultValue ...string) string {
	if val, ok := e.overrides[key]; ok {
		return val
	}
	if val := os.Getenv(key); val != "" {
		return val
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

// Set overrides an environment variable.
func (e *EnvManager) Set(key string, value string) {
	e.overrides[key] = value
}

// GetOrFail returns the value or panics if not found.
func (e *EnvManager) GetOrFail(key string) string {
	val := e.Get(key)
	if val == "" {
		panic("Missing required environment variable: " + key)
	}
	return val
}
