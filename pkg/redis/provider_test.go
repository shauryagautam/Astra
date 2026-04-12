package redis

import (
	"testing"

	"github.com/shauryagautam/Astra/pkg/test_util"
	"github.com/stretchr/testify/assert"
)

func TestRedisProvider_Register(t *testing.T) {
	ta := test_util.NewTestApp(t, nil)
	app := ta.App

	// Set Redis connection to empty string to avoid real connection attempt
	app.Config().Redis.Host = ""
	app.Config().Redis.URL = ""

	p := &RedisProvider{}
	err := p.Register(app)
	assert.NoError(t, err)
}

func TestRedisProvider_IsOptIn(t *testing.T) {
	ta := test_util.NewTestApp(t, nil)
	app := ta.App

	// Phase 1.2: Redis as Opt-in Provider
	app.Config().Redis.Host = ""
	app.Config().Redis.URL = ""

	p := &RedisProvider{}
	// Boot connects and pings, but it should return early if not configured
	err := p.Boot(app)
	assert.NoError(t, err)
}
