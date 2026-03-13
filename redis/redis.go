package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/events"
	"github.com/redis/go-redis/v9"
)

// Manager handles multiple Redis connections and their lifecycle.
type Manager struct {
	configs map[string]config.RedisConfig
	clients map[string]*Client
	events  *events.Emitter
	mu      sync.RWMutex
	started bool
}

// NewManager creates a new Redis manager with the given initial config for the "default" connection.
func NewManager(cfg config.RedisConfig, emitter *events.Emitter) *Manager {
	return &Manager{
		configs: map[string]config.RedisConfig{"default": cfg},
		clients: make(map[string]*Client),
		events:  emitter,
	}
}

// AddConfig adds a named Redis configuration.
func (m *Manager) AddConfig(name string, cfg config.RedisConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[name] = cfg
}

// Client returns the default Redis client.
func (m *Manager) Client() (*Client, error) {
	return m.Default()
}

// Default returns the "default" Redis client.
func (m *Manager) Default() (*Client, error) {
	return m.Connection("default")
}

// Connection returns a named Redis connection. It connects if not already started.
func (m *Manager) Connection(name string) (*Client, error) {
	m.mu.RLock()
	client, ok := m.clients[name]
	m.mu.RUnlock()

	if ok {
		return client, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check again in case another goroutine connected it
	if client, ok := m.clients[name]; ok {
		return client, nil
	}

	cfg, ok := m.configs[name]
	if !ok {
		return nil, fmt.Errorf("redis: connection %q not configured", name)
	}

	newClient, err := NewClient(cfg, m.events)
	if err != nil {
		return nil, err
	}

	m.clients[name] = newClient
	return newClient, nil
}

// Connect initializes and pings all configured Redis connections.
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil
	}

	for name, cfg := range m.configs {
		if _, ok := m.clients[name]; ok {
			continue
		}
		client, err := NewClient(cfg, m.events)
		if err != nil {
			return fmt.Errorf("redis: failed to connect to %q: %w", name, err)
		}
		m.clients[name] = client
	}

	m.started = true
	return nil
}

// Close gracefully closes all active Redis connections.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for name, client := range m.clients {
		if err := client.Stop(ctx); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("redis: connection %q close failed: %w", name, err)
			}
		}
	}
	m.clients = make(map[string]*Client)
	m.started = false
	return firstErr
}

// Client wraps the go-redis UniversalClient and adds Astra-specific advanced features.
type Client struct {
	redis.UniversalClient
	config config.RedisConfig
	events *events.Emitter
	mu     sync.RWMutex
	// Pipeline buffer for ultra-fast batch operations
	pipeline  redis.Pipeliner
	batchSize int
}

// Name returns the service name.
func (c *Client) Name() string {
	return "redis"
}

// PipelineBatch executes commands in a pipeline for ultra-fast performance
func (c *Client) PipelineBatch(ctx context.Context, cmds ...redis.Cmder) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create pipeline if not exists
	if c.pipeline == nil {
		c.pipeline = c.UniversalClient.Pipeline()
		c.batchSize = 0
	}

	// Add commands to pipeline
	for _, cmd := range cmds {
		c.pipeline.Process(ctx, cmd)
		c.batchSize++
	}

	// Execute pipeline if batch size reached threshold
	if c.batchSize >= 100 { // Ultra-fast batch size
		_, err := c.pipeline.Exec(ctx)
		c.pipeline = c.UniversalClient.Pipeline()
		c.batchSize = 0
		return err
	}

	return nil
}

// FlushPipeline executes any pending pipeline operations
func (c *Client) FlushPipeline(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipeline != nil && c.batchSize > 0 {
		_, err := c.pipeline.Exec(ctx)
		c.pipeline = c.UniversalClient.Pipeline()
		c.batchSize = 0
		return err
	}
	return nil
}

// MSetBatch ultra-fast multiple set operation
func (c *Client) MSetBatch(ctx context.Context, pairs ...string) error {
	if len(pairs)%2 != 0 {
		return fmt.Errorf("redis: MSetBatch requires even number of arguments")
	}

	// Use pipeline for ultra-fast batch sets
	pipe := c.UniversalClient.Pipeline()
	for i := 0; i < len(pairs); i += 2 {
		pipe.Set(ctx, pairs[i], pairs[i+1], 0)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// MGetBatch ultra-fast multiple get operation
func (c *Client) MGetBatch(ctx context.Context, keys ...string) ([]interface{}, error) {
	return c.UniversalClient.MGet(ctx, keys...).Result()
}

// Stop closes the underlying Redis client.
func (c *Client) Stop(ctx context.Context) error {
	// Flush any pending pipeline operations before closing
	_ = c.FlushPipeline(ctx)

	if c.UniversalClient != nil {
		return c.UniversalClient.Close()
	}
	return nil
}

// NewClient creates a new Astra Redis client with ultra-fast optimizations.
func NewClient(cfg config.RedisConfig, emitter *events.Emitter) (*Client, error) {
	var addrs []string
	var db int
	var password string

	if cfg.URL != "" {
		opt, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("redis: failed to parse URL: %w", err)
		}
		addrs = []string{opt.Addr}
		db = opt.DB
		password = opt.Password
	} else {
		if cfg.UseSentinel {
			addrs = cfg.SentinelAddrs
		} else {
			addrs = []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)}
		}
		db = cfg.DB
		password = cfg.Password
	}

	// Ultra-fast Redis connection optimizations
	opts := &redis.UniversalOptions{
		Addrs:           addrs,
		Password:        password,
		DB:              db,
		MaxRetries:      0, // Disable retries for ultra-fast performance
		PoolSize:        cfg.PoolSize,
		DialTimeout:     2 * time.Second, // Faster connection
		ReadTimeout:     1 * time.Second, // Faster reads
		WriteTimeout:    1 * time.Second, // Faster writes
		PoolTimeout:     2 * time.Second, // Faster pool acquisition
		ConnMaxIdleTime: 5 * time.Minute, // Aggressive cleanup
		MinIdleConns:    10,              // Maintain warm connections
	}

	if cfg.PoolSize == 0 {
		opts.PoolSize = 200 // High-performance default
	}

	if cfg.UseSentinel {
		opts.MasterName = cfg.SentinelMaster
	}

	client := redis.NewUniversalClient(opts)

	// Add event emission hook
	if emitter != nil {
		client.AddHook(&redisHook{events: emitter})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close() // #nosec G104
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	return &Client{
		UniversalClient: client,
		config:          cfg,
		events:          emitter,
	}, nil
}

type redisHook struct {
	events *events.Emitter
}

func (h *redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *redisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		duration := time.Since(start)

		if h.events != nil {
			h.events.EmitPayload(ctx, "redis.command_executed", map[string]any{
				"command":  cmd.Name(),
				"args":     cmd.Args(),
				"duration": duration,
				"error":    err,
			})
		}
		return err
	}
}

func (h *redisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		duration := time.Since(start)

		if h.events != nil {
			names := make([]string, len(cmds))
			for i, cmd := range cmds {
				names[i] = cmd.Name()
			}
			h.events.EmitPayload(ctx, "redis.pipeline_executed", map[string]any{
				"commands": names,
				"count":    len(cmds),
				"duration": duration,
				"error":    err,
			})
		}
		return err
	}
}

// HealthCheck verifies that the Redis client can respond to a PING.
func HealthCheck(ctx context.Context, client redis.UniversalClient) error {
	if client == nil {
		return fmt.Errorf("redis: client is nil")
	}
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: ping failed: %w", err)
	}
	return nil
}
