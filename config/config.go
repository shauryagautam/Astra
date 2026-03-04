// Package config provides typed configuration loading for Astra applications.
// It loads environment variables from .env files and provides type-safe getters.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds loaded environment variables and provides typed access.
type Config struct {
	env map[string]string
}

// Load creates a new Config by loading environment variables from the given
// .env file paths. If no paths are provided, it attempts to load ".env".
// Environment variables already set in the system take precedence.
func Load(paths ...string) (*Config, error) {
	if len(paths) == 0 {
		paths = []string{".env"}
	}

	// godotenv does NOT override existing env vars
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err != nil {
				return nil, fmt.Errorf("config.Load: failed to load %s: %w", path, err)
			}
		}
	}

	return &Config{env: envToMap()}, nil
}

// MustLoad is like Load but panics on error.
func MustLoad(paths ...string) *Config {
	cfg, err := Load(paths...)
	if err != nil {
		panic(fmt.Sprintf("config.MustLoad: %v", err))
	}
	return cfg
}

// String returns the value for the given key, or the default if not set.
func (c *Config) String(key string, def string) string {
	if val, ok := c.env[key]; ok && val != "" {
		return val
	}
	// Also check live env (in case it was set after Load)
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

// Int returns the integer value for the given key, or the default if not set or invalid.
func (c *Config) Int(key string, def int) int {
	raw := c.String(key, "")
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return val
}

// Int32 returns the int32 value for the given key, or the default if not set or invalid.
func (c *Config) Int32(key string, def int32) int32 {
	raw := c.String(key, "")
	if raw == "" {
		return def
	}
	val, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return def
	}
	return int32(val)
}

// Bool returns the boolean value for the given key, or the default if not set.
// Truthy values: "true", "1", "yes", "on" (case-insensitive).
func (c *Config) Bool(key string, def bool) bool {
	raw := c.String(key, "")
	if raw == "" {
		return def
	}
	switch strings.ToLower(raw) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return def
	}
}

// Duration returns the time.Duration value for the given key, or the default.
// Accepts Go duration strings like "30s", "5m", "1h".
func (c *Config) Duration(key string, def time.Duration) time.Duration {
	raw := c.String(key, "")
	if raw == "" {
		return def
	}
	val, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return val
}

// IsDev returns true if APP_ENV is "development" or "dev".
func (c *Config) IsDev() bool {
	env := strings.ToLower(c.String("APP_ENV", "development"))
	return env == "development" || env == "dev"
}

// IsProd returns true if APP_ENV is "production" or "prod".
func (c *Config) IsProd() bool {
	env := strings.ToLower(c.String("APP_ENV", "development"))
	return env == "production" || env == "prod"
}

// IsTest returns true if APP_ENV is "test" or "testing".
func (c *Config) IsTest() bool {
	env := strings.ToLower(c.String("APP_ENV", "development"))
	return env == "test" || env == "testing"
}

// envToMap returns all current environment variables as a map.
func envToMap() map[string]string {
	m := make(map[string]string)
	for _, entry := range os.Environ() {
		k, v, _ := strings.Cut(entry, "=")
		m[k] = v
	}
	return m
}
