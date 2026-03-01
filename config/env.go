// Package config provides environment variable loading and typed configuration access.
// Loads .env files on application startup, mirroring Astra's Env module.
//
// Usage:
//
//	config.LoadEnv(".env")               // loads .env file into os environment
//	config.LoadEnv(".env.production")    // override with production settings
//
// The .env file format supports:
//   - KEY=value
//   - KEY="quoted value"
//   - KEY='single quoted value'
//   - # comments
//   - Empty lines
//   - Variable expansion: KEY=${OTHER_KEY}
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadEnv loads a .env file and sets the values in the process environment.
// Existing environment variables are NOT overwritten (real env takes precedence).
func LoadEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // .env file is optional
		}
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle export prefix: export KEY=value
		line = strings.TrimPrefix(line, "export ")

		// Split on first =
		idx := strings.IndexByte(line, '=')
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove surrounding quotes
		value = unquote(value)

		// Expand variables: ${VAR_NAME} or $VAR_NAME
		value = expandVariables(value)

		// Don't overwrite existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// LoadEnvOverride loads a .env file, overwriting existing variables.
func LoadEnvOverride(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		idx := strings.IndexByte(line, '=')
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		value = unquote(value)
		value = expandVariables(value)
		os.Setenv(key, value)
	}

	return scanner.Err()
}

// ══════════════════════════════════════════════════════════════════════
// Typed Environment Getters
// ══════════════════════════════════════════════════════════════════════

// EnvGet returns an environment variable value or a default.
func EnvGet(key string, defaultValue ...string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

// EnvGetOrFail returns an environment variable value or panics.
func EnvGetOrFail(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("Missing required environment variable: %s", key))
	}
	return val
}

// EnvGetInt returns an environment variable as an integer.
func EnvGetInt(key string, defaultValue ...int) int {
	val := os.Getenv(key)
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

// EnvGetBool returns an environment variable as a boolean.
func EnvGetBool(key string, defaultValue ...bool) bool {
	val := os.Getenv(key)
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

// EnvGetDuration returns an environment variable as a time.Duration.
// Accepts formats like "5s", "10m", "1h".
func EnvGetDuration(key string, defaultValue ...time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return 0
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return 0
	}
	return d
}

// EnvGetFloat returns an environment variable as a float64.
func EnvGetFloat(key string, defaultValue ...float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return 0
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return 0
	}
	return f
}

// ══════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════

// unquote removes surrounding quotes from a value.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// expandVariables expands ${VAR} and $VAR references in a string.
func expandVariables(s string) string {
	return os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})
}
