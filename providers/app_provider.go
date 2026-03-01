package providers

import (
	"log"
	"os"
	"strconv"

	"github.com/shaurya/astra/config"
	"github.com/shaurya/astra/contracts"
)

// AppProvider registers core application services into the container.
// This is the first provider registered, wiring up Logger, Env, etc.
// Mirrors Astra's AppProvider.
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
	// Load .env file on startup
	config.LoadEnv(".env") //nolint:errcheck

	// Register the Logger
	p.App.Singleton("Astra/Core/Logger", func(c contracts.ContainerContract) (any, error) {
		return log.New(os.Stdout, "[astra] ", log.LstdFlags), nil
	})
	p.App.Alias("Logger", "Astra/Core/Logger")

	// Register the Env manager
	p.App.Singleton("Astra/Core/Env", func(c contracts.ContainerContract) (any, error) {
		return NewEnvManager(), nil
	})
	p.App.Alias("Env", "Astra/Core/Env")

	return nil
}

// EnvManager handles environment variable access.
// Mirrors Astra's Env module.
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

// GetInt returns an environment variable as an integer.
func (e *EnvManager) GetInt(key string, defaultValue ...int) int {
	val := e.Get(key)
	if val == "" {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return 0
	}
	return n
}

// GetBool returns an environment variable as a boolean.
func (e *EnvManager) GetBool(key string, defaultValue ...bool) bool {
	val := e.Get(key)
	if val == "" {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return false
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return false
	}
	return b
}
