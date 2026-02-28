// Package redis provides the Redis module for Adonis Go.
// Wraps go-redis with an AdonisJS-style API supporting multiple connections,
// pub/sub, caching, rate limiting, and session storage.
package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shaurya/adonis/contracts"
)

// ConnectionConfig holds settings for a Redis connection.
type ConnectionConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// ManagerConfig holds all Redis connections config.
type ManagerConfig struct {
	Default     string
	Connections map[string]ConnectionConfig
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Default: "local",
		Connections: map[string]ConnectionConfig{
			"local": {Host: "127.0.0.1", Port: 6379, Password: "", DB: 0},
		},
	}
}

// Manager manages multiple Redis connections.
// Mirrors AdonisJS: Redis.connection('local').get('key')
type Manager struct {
	mu          sync.RWMutex
	config      ManagerConfig
	connections map[string]*Connection
}

// NewManager creates a new Redis manager.
func NewManager(config ManagerConfig) *Manager {
	return &Manager{
		config:      config,
		connections: make(map[string]*Connection),
	}
}

// Connection returns a named Redis connection (creates lazily).
func (m *Manager) Connection(name string) contracts.RedisConnectionContract {
	m.mu.RLock()
	if conn, ok := m.connections[name]; ok {
		m.mu.RUnlock()
		return conn
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check
	if conn, ok := m.connections[name]; ok {
		return conn
	}

	cfg, ok := m.config.Connections[name]
	if !ok {
		panic(fmt.Sprintf("Redis connection '%s' not configured", name))
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	conn := &Connection{client: client}
	m.connections[name] = conn
	return conn
}

// Default returns the default connection.
func (m *Manager) Default() contracts.RedisConnectionContract {
	return m.Connection(m.config.Default)
}

// Quit closes all connections.
func (m *Manager) Quit() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, conn := range m.connections {
		if err := conn.client.Close(); err != nil {
			return fmt.Errorf("failed to close Redis connection '%s': %w", name, err)
		}
	}
	m.connections = make(map[string]*Connection)
	return nil
}

var _ contracts.RedisContract = (*Manager)(nil)

// ══════════════════════════════════════════════════════════════════════
// Connection — wraps a single go-redis client
// ══════════════════════════════════════════════════════════════════════

// Connection wraps a go-redis client with the contracts interface.
type Connection struct {
	client *goredis.Client
}

// Client returns the underlying go-redis client for advanced usage.
func (c *Connection) Client() *goredis.Client {
	return c.client
}

// --- String Commands ---

func (c *Connection) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *Connection) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

func (c *Connection) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *Connection) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *Connection) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c *Connection) Decr(ctx context.Context, key string) (int64, error) {
	return c.client.Decr(ctx, key).Result()
}

func (c *Connection) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

func (c *Connection) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

func (c *Connection) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// --- Hash Commands ---

func (c *Connection) HGet(ctx context.Context, key string, field string) (string, error) {
	return c.client.HGet(ctx, key, field).Result()
}

func (c *Connection) HSet(ctx context.Context, key string, field string, value any) error {
	return c.client.HSet(ctx, key, field, value).Err()
}

func (c *Connection) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, key, fields...).Err()
}

func (c *Connection) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

// --- List Commands ---

func (c *Connection) LPush(ctx context.Context, key string, values ...any) error {
	return c.client.LPush(ctx, key, values...).Err()
}

func (c *Connection) RPush(ctx context.Context, key string, values ...any) error {
	return c.client.RPush(ctx, key, values...).Err()
}

func (c *Connection) LPop(ctx context.Context, key string) (string, error) {
	return c.client.LPop(ctx, key).Result()
}

func (c *Connection) RPop(ctx context.Context, key string) (string, error) {
	return c.client.RPop(ctx, key).Result()
}

func (c *Connection) LLen(ctx context.Context, key string) (int64, error) {
	return c.client.LLen(ctx, key).Result()
}

func (c *Connection) LRange(ctx context.Context, key string, start int64, stop int64) ([]string, error) {
	return c.client.LRange(ctx, key, start, stop).Result()
}

func (c *Connection) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	return c.client.BRPop(ctx, timeout, keys...).Result()
}

// --- Set Commands ---

func (c *Connection) SAdd(ctx context.Context, key string, members ...any) error {
	return c.client.SAdd(ctx, key, members...).Err()
}

func (c *Connection) SRem(ctx context.Context, key string, members ...any) error {
	return c.client.SRem(ctx, key, members...).Err()
}

func (c *Connection) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.client.SMembers(ctx, key).Result()
}

func (c *Connection) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return c.client.SIsMember(ctx, key, member).Result()
}

// --- Sorted Set Commands ---

func (c *Connection) ZAdd(ctx context.Context, key string, score float64, member any) error {
	return c.client.ZAdd(ctx, key, goredis.Z{Score: score, Member: member}).Err()
}

func (c *Connection) ZRangeByScore(ctx context.Context, key string, min string, max string) ([]string, error) {
	return c.client.ZRangeByScore(ctx, key, &goredis.ZRangeBy{
		Min: min,
		Max: max,
	}).Result()
}

func (c *Connection) ZRem(ctx context.Context, key string, members ...any) error {
	return c.client.ZRem(ctx, key, members...).Err()
}

// --- Pub/Sub ---

func (c *Connection) Publish(ctx context.Context, channel string, message any) error {
	return c.client.Publish(ctx, channel, message).Err()
}

func (c *Connection) Subscribe(ctx context.Context, channels ...string) contracts.PubSubContract {
	sub := c.client.Subscribe(ctx, channels...)
	return &PubSub{sub: sub, ctx: ctx}
}

// --- Misc ---

func (c *Connection) FlushDB(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

func (c *Connection) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Connection) Pipeline() contracts.RedisPipeContract {
	return &Pipe{pipe: c.client.Pipeline()}
}

var _ contracts.RedisConnectionContract = (*Connection)(nil)

// ══════════════════════════════════════════════════════════════════════
// Pipe — wraps go-redis pipeliner
// ══════════════════════════════════════════════════════════════════════

type Pipe struct {
	pipe goredis.Pipeliner
}

func (p *Pipe) LPush(ctx context.Context, key string, values ...any) {
	p.pipe.LPush(ctx, key, values...)
}

func (p *Pipe) ZRem(ctx context.Context, key string, members ...any) {
	p.pipe.ZRem(ctx, key, members...)
}

func (p *Pipe) Exec(ctx context.Context) error {
	_, err := p.pipe.Exec(ctx)
	return err
}

var _ contracts.RedisPipeContract = (*Pipe)(nil)

// ══════════════════════════════════════════════════════════════════════
// PubSub
// ══════════════════════════════════════════════════════════════════════

// PubSub wraps go-redis pub/sub with the contracts interface.
type PubSub struct {
	sub  *goredis.PubSub
	ctx  context.Context
	ch   <-chan contracts.PubSubMessage
	once sync.Once
}

// Channel returns a Go channel that receives pub/sub messages.
func (ps *PubSub) Channel() <-chan contracts.PubSubMessage {
	ps.once.Do(func() {
		goCh := ps.sub.Channel()
		ch := make(chan contracts.PubSubMessage)
		go func() {
			defer close(ch)
			for msg := range goCh {
				ch <- contracts.PubSubMessage{
					Channel: msg.Channel,
					Payload: msg.Payload,
				}
			}
		}()
		ps.ch = ch
	})
	return ps.ch
}

// Close unsubscribes and closes.
func (ps *PubSub) Close() error {
	return ps.sub.Close()
}

var _ contracts.PubSubContract = (*PubSub)(nil)
