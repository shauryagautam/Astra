package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shaurya/adonis/contracts"
)

// Cache provides a Redis-backed cache driver.
// Mirrors AdonisJS's Cache module.
//
// Usage:
//
//	cache := redis.NewCache(redisConn, "cache:")
//	cache.Put(ctx, "user:1", userData, 10*time.Minute)
//	val, _ := cache.Get(ctx, "user:1")
type Cache struct {
	conn   contracts.RedisConnectionContract
	prefix string
}

// NewCache creates a new Redis-backed cache.
func NewCache(conn contracts.RedisConnectionContract, prefix string) *Cache {
	if prefix == "" {
		prefix = "adonis:cache:"
	}
	return &Cache{conn: conn, prefix: prefix}
}

func (c *Cache) key(k string) string {
	return c.prefix + k
}

// Get retrieves a cached value.
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.conn.Get(ctx, c.key(key))
}

// Put stores a value with a TTL.
func (c *Cache) Put(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := serialize(value)
	if err != nil {
		return err
	}
	return c.conn.Set(ctx, c.key(key), data, ttl)
}

// Has checks if a key exists.
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	return c.conn.Exists(ctx, c.key(key))
}

// Forget removes a cached key.
func (c *Cache) Forget(ctx context.Context, key string) error {
	return c.conn.Del(ctx, c.key(key))
}

// Forever stores a value that never expires.
func (c *Cache) Forever(ctx context.Context, key string, value any) error {
	return c.Put(ctx, key, value, 0)
}

// Flush clears all cached data (WARNING: flushes entire Redis DB!).
func (c *Cache) Flush(ctx context.Context) error {
	return c.conn.FlushDB(ctx)
}

// Remember gets a cached value or executes the callback and caches the result.
// Mirrors: Cache.remember('key', '10m', async () => { ... })
func (c *Cache) Remember(ctx context.Context, key string, ttl time.Duration, callback func() (any, error)) (string, error) {
	// Try to get cached value
	val, err := c.Get(ctx, key)
	if err == nil && val != "" {
		return val, nil
	}

	// Execute callback
	result, err := callback()
	if err != nil {
		return "", err
	}

	// Cache the result
	if err := c.Put(ctx, key, result, ttl); err != nil {
		return "", err
	}

	data, _ := serialize(result)
	return data, nil
}

// Increment increments a numeric value.
func (c *Cache) Increment(ctx context.Context, key string, value ...int64) (int64, error) {
	v := int64(1)
	if len(value) > 0 {
		v = value[0]
	}
	return c.conn.IncrBy(ctx, c.key(key), v)
}

// Decrement decrements a numeric value.
func (c *Cache) Decrement(ctx context.Context, key string, value ...int64) (int64, error) {
	v := int64(1)
	if len(value) > 0 {
		v = value[0]
	}
	return c.conn.IncrBy(ctx, c.key(key), -v)
}

var _ contracts.CacheContract = (*Cache)(nil)

// ══════════════════════════════════════════════════════════════════════
// Rate Limiter
// Uses Redis sliding window for rate limiting.
// Mirrors AdonisJS's ThrottleMiddleware under the hood.
// ══════════════════════════════════════════════════════════════════════

// RateLimiter provides Redis-backed rate limiting.
//
// Usage:
//
//	limiter := redis.NewRateLimiter(redisConn, "ratelimit:")
//	allowed, _ := limiter.Attempt(ctx, "api:"+userIP, 60, time.Minute)
//	if !allowed {
//	    return ctx.Response().Status(429).Json(...)
//	}
type RateLimiter struct {
	conn   contracts.RedisConnectionContract
	prefix string
}

// NewRateLimiter creates a new Redis-backed rate limiter.
func NewRateLimiter(conn contracts.RedisConnectionContract, prefix string) *RateLimiter {
	if prefix == "" {
		prefix = "adonis:ratelimit:"
	}
	return &RateLimiter{conn: conn, prefix: prefix}
}

func (r *RateLimiter) key(k string) string {
	return r.prefix + k
}

// Attempt checks if the key has exceeded max attempts within the window.
// Returns true if the request is allowed.
func (r *RateLimiter) Attempt(ctx context.Context, key string, maxAttempts int, window time.Duration) (bool, error) {
	k := r.key(key)

	// Increment the counter
	count, err := r.conn.Incr(ctx, k)
	if err != nil {
		return false, err
	}

	// Set expiry on first attempt
	if count == 1 {
		if err := r.conn.Expire(ctx, k, window); err != nil {
			return false, err
		}
	}

	return count <= int64(maxAttempts), nil
}

// TooManyAttempts returns true if the key has exceeded max attempts.
func (r *RateLimiter) TooManyAttempts(ctx context.Context, key string, maxAttempts int) (bool, error) {
	val, err := r.conn.Get(ctx, r.key(key))
	if err != nil {
		return false, nil // Key doesn't exist = not throttled
	}

	var count int64
	fmt.Sscanf(val, "%d", &count)
	return count >= int64(maxAttempts), nil
}

// RemainingAttempts returns the number of remaining attempts.
func (r *RateLimiter) RemainingAttempts(ctx context.Context, key string, maxAttempts int) (int, error) {
	val, err := r.conn.Get(ctx, r.key(key))
	if err != nil {
		return maxAttempts, nil
	}

	var count int64
	fmt.Sscanf(val, "%d", &count)
	remaining := int64(maxAttempts) - count
	if remaining < 0 {
		remaining = 0
	}
	return int(remaining), nil
}

// Clear resets the counter for a key.
func (r *RateLimiter) Clear(ctx context.Context, key string) error {
	return r.conn.Del(ctx, r.key(key))
}

var _ contracts.RateLimiterContract = (*RateLimiter)(nil)

// ══════════════════════════════════════════════════════════════════════
// Session Store
// Redis-backed session storage.
// ══════════════════════════════════════════════════════════════════════

// SessionStore provides Redis-backed session storage.
type SessionStore struct {
	conn   contracts.RedisConnectionContract
	prefix string
	ttl    time.Duration
}

// NewSessionStore creates a new Redis-backed session store.
func NewSessionStore(conn contracts.RedisConnectionContract, ttl time.Duration) *SessionStore {
	return &SessionStore{
		conn:   conn,
		prefix: "adonis:session:",
		ttl:    ttl,
	}
}

// Get retrieves session data by session ID.
func (s *SessionStore) Get(ctx context.Context, sessionID string) (map[string]any, error) {
	data, err := s.conn.Get(ctx, s.prefix+sessionID)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Set stores session data.
func (s *SessionStore) Set(ctx context.Context, sessionID string, data map[string]any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.conn.Set(ctx, s.prefix+sessionID, string(jsonData), s.ttl)
}

// Destroy removes a session.
func (s *SessionStore) Destroy(ctx context.Context, sessionID string) error {
	return s.conn.Del(ctx, s.prefix+sessionID)
}

// Regenerate creates a new session ID while keeping the data.
func (s *SessionStore) Regenerate(ctx context.Context, oldID string, newID string) error {
	data, err := s.Get(ctx, oldID)
	if err != nil {
		return err
	}

	if err := s.Set(ctx, newID, data); err != nil {
		return err
	}

	return s.Destroy(ctx, oldID)
}

// Touch extends the session TTL.
func (s *SessionStore) Touch(ctx context.Context, sessionID string) error {
	return s.conn.Expire(ctx, s.prefix+sessionID, s.ttl)
}

// ══════════════════════════════════════════════════════════════════════
// Redis Token Store (for OAT guard)
// ══════════════════════════════════════════════════════════════════════

// TokenStore is a Redis-backed token store for the OAT auth guard.
// Replaces the in-memory store for production use.
type TokenStore struct {
	conn   contracts.RedisConnectionContract
	prefix string
}

// NewTokenStore creates a Redis-backed token store.
func NewTokenStore(conn contracts.RedisConnectionContract) *TokenStore {
	return &TokenStore{
		conn:   conn,
		prefix: "adonis:token:",
	}
}

// Store saves a token associated with a user.
func (s *TokenStore) Store(userID string, token string, name string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	ctx := context.Background()
	data, _ := json.Marshal(map[string]string{
		"user_id": userID,
		"name":    name,
	})

	// Store token -> user mapping
	if err := s.conn.Set(ctx, s.prefix+token, string(data), ttl); err != nil {
		return err
	}

	// Also store in user's token set for revoke-all
	return s.conn.SAdd(ctx, s.prefix+"user:"+userID, token)
}

// Find looks up a token and returns the associated user ID.
func (s *TokenStore) Find(token string) (string, error) {
	ctx := context.Background()
	data, err := s.conn.Get(ctx, s.prefix+token)
	if err != nil {
		return "", err
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return "", err
	}

	return result["user_id"], nil
}

// Revoke deletes a specific token.
func (s *TokenStore) Revoke(token string) error {
	ctx := context.Background()
	// Get user ID first to clean up the user's token set
	userID, err := s.Find(token)
	if err == nil && userID != "" {
		s.conn.SRem(ctx, s.prefix+"user:"+userID, token) //nolint:errcheck
	}
	return s.conn.Del(ctx, s.prefix+token)
}

// RevokeAll deletes all tokens for a user.
func (s *TokenStore) RevokeAll(userID string) error {
	ctx := context.Background()
	tokens, err := s.conn.SMembers(ctx, s.prefix+"user:"+userID)
	if err != nil {
		return err
	}

	for _, token := range tokens {
		s.conn.Del(ctx, s.prefix+token) //nolint:errcheck
	}

	return s.conn.Del(ctx, s.prefix+"user:"+userID)
}

// ══════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════

func serialize(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to serialize value: %w", err)
		}
		return string(data), nil
	}
}
