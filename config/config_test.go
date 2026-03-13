package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Load(t *testing.T) {
	// Set some env vars
	os.Setenv("APP_NAME", "AstraTest")
	os.Setenv("APP_ENV", "testing")
	defer os.Unsetenv("APP_NAME")
	defer os.Unsetenv("APP_ENV")

	// Load without paths will try loading .env
	cfg, err := Load()
	assert.NoError(t, err)

	// Test the String getter
	assert.Equal(t, "AstraTest", cfg.String("APP_NAME", ""))
	assert.Equal(t, "testing", cfg.String("APP_ENV", ""))
}

func TestConfig_LoadFromEnv(t *testing.T) {
	// Since we can't easily mock the internal map, we test LoadFromEnv with actual env if possible
	// Or we just test that it populates AstraConfig
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	defer os.Unsetenv("DATABASE_URL")

	env, _ := Load()
	astraCfg := LoadFromEnv(env)
	assert.Equal(t, "postgres://localhost/test", astraCfg.Database.URL)
}
