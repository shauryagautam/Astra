package config

// AppConfig holds the main application configuration.
// Mirrors AdonisJS's config/app.ts.
type AppConfig struct {
	// Name is the application name.
	Name string

	// Environment: "development", "production", "test"
	Environment string

	// Port is the HTTP server port.
	Port int

	// Host is the HTTP server host.
	Host string

	// AppKey is the secret key used for encryption/hashing.
	AppKey string

	// Debug enables debug mode with detailed error pages.
	Debug bool
}

// DefaultAppConfig returns sensible defaults.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Name:        "Adonis",
		Environment: "development",
		Port:        3333,
		Host:        "0.0.0.0",
		Debug:       true,
	}
}
