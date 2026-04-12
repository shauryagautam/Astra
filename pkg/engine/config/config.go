// Package config provides typed configuration loading for Astra applications.
// It loads environment variables from .env files, YAML, and TOML and provides type-safe getters.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Config holds loaded configuration and provides typed access.
type Config struct {
	data map[string]any
}

// Load creates a new Config by loading configuration from .env, YAML, and TOML files.
// Priority (highest wins): Env vars > .env > YAML > TOML.
func Load(paths ...string) (*Config, error) {
	c := &Config{data: make(map[string]any)}

	// 1. Load from TOML (lowest priority)
	if err := c.loadFiles("config/*.toml", toml.Unmarshal); err != nil {
		return nil, err
	}

	// 2. Load from YAML
	if err := c.loadFiles("config/*.yaml", yaml.Unmarshal); err != nil {
		return nil, err
	}
	if err := c.loadFiles("config/*.yml", yaml.Unmarshal); err != nil {
		return nil, err
	}

	// 3. Load from .env
	if len(paths) == 0 {
		paths = []string{".env"}
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			env, err := godotenv.Read(path)
			if err != nil {
				return nil, fmt.Errorf("config.Load: failed to read %s: %w", path, err)
			}
			for k, v := range env {
				c.data[k] = v
			}
			// Also load into process environment
			_ = godotenv.Load(path)
		}
	}

	// 4. Load from process environment (highest priority)
	for _, entry := range os.Environ() {
		k, v, _ := strings.Cut(entry, "=")
		c.data[k] = v
	}

	return c, nil
}

// loadFiles finds files matching pattern and unmarshals them into c.data.
func (c *Config) loadFiles(pattern string, unmarshal func([]byte, any) error) error {
	matches, _ := filepath.Glob(pattern)
	for _, path := range matches {
		buf, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("config: failed to read %s: %w", path, err)
		}
		var m map[string]any
		if err := unmarshal(buf, &m); err != nil {
			return fmt.Errorf("config: failed to parse %s: %w", path, err)
		}
		for k, v := range m {
			c.data[strings.ToUpper(k)] = v
		}
	}
	return nil
}

// Get retrieves a configuration value and casts it to T.
func Get[T any](c *Config, key string) T {
	val, ok := c.data[strings.ToUpper(key)]
	if !ok {
		var zero T
		return zero
	}

	if typed, ok := val.(T); ok {
		return typed
	}

	// Try string conversion if the target is a different type
	str := fmt.Sprint(val)
	var res any
	var err error

	var target T
	switch any(target).(type) {
	case string:
		res = str
	case int:
		res, err = strconv.Atoi(str)
	case int64:
		res, err = strconv.ParseInt(str, 10, 64)
	case bool:
		res, err = strconv.ParseBool(str)
	case time.Duration:
		res, err = time.ParseDuration(str)
	}

	if err == nil && res != nil {
		return res.(T)
	}

	var zero T
	return zero
}

// String returns the value for the given key, or the default if not set.
func (c *Config) String(key string, def string) string {
	res := Get[string](c, key)
	if res == "" {
		return def
	}
	return res
}

// Int returns the integer value for the given key, or the default.
func (c *Config) Int(key string, def int) int {
	val := Get[any](c, key)
	if val == nil {
		return def
	}
	str := fmt.Sprint(val)
	res, err := strconv.Atoi(str)
	if err != nil {
		return def
	}
	return res
}

// Int32 returns the int32 value for the given key, or the default.
func (c *Config) Int32(key string, def int32) int32 {
	val := Get[any](c, key)
	if val == nil {
		return def
	}
	str := fmt.Sprint(val)
	res, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		return def
	}
	return int32(res)
}

// Bool returns the boolean value for the given key, or the default.
func (c *Config) Bool(key string, def bool) bool {
	val := Get[any](c, key)
	if val == nil {
		return def
	}
	str := strings.ToLower(fmt.Sprint(val))
	switch str {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return def
	}
}

// Duration returns the time.Duration value for the given key, or the default.
func (c *Config) Duration(key string, def time.Duration) time.Duration {
	val := Get[any](c, key)
	if val == nil {
		return def
	}
	str := fmt.Sprint(val)
	res, err := time.ParseDuration(str)
	if err != nil {
		return def
	}
	return res
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

// MaskSecrets returns a copy of the config data with sensitive values masked.
func (c *Config) MaskSecrets() map[string]any {
	masked := make(map[string]any)
	secrets := []string{"SECRET", "PASSWORD", "TOKEN", "KEY"}
	for k, v := range c.data {
		isSecret := false
		for _, s := range secrets {
			if strings.Contains(strings.ToUpper(k), s) {
				isSecret = true
				break
			}
		}
		if isSecret {
			masked[k] = "********"
		} else {
			masked[k] = v
		}
	}
	return masked
}

// Raw returns the underlying config map.
func (c *Config) Raw() map[string]any {
	return c.data
}
