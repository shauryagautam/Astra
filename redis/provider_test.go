package redis

import (
	"testing"

	"github.com/astraframework/astra/core"
	"github.com/stretchr/testify/assert"
)

func TestRedisProvider_Register(t *testing.T) {
	app, err := core.New()
	assert.NoError(t, err)

	// Set Redis connection to empty string to avoid real connection attempt
	app.Config.Redis.Host = ""
	app.Config.Redis.URL = ""

	p := &RedisProvider{}
	err = p.Register(app)
	assert.NoError(t, err)
}

func TestRedisProvider_IsOptIn(t *testing.T) {
	app, err := core.New()
	assert.NoError(t, err)

	// Phase 1.2: Redis as Opt-in Provider
	app.Config.Redis.Host = ""
	app.Config.Redis.URL = ""

	p := &RedisProvider{}
	err = p.Boot(app)
	assert.NoError(t, err)
	assert.False(t, app.Has("redis"), "Redis should not be registered if connection is empty")
}
