package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a server-side session store backed by Redis.
// It stores serialised session data in Redis under a key of the form:
//
//	{prefix}:{sessionID}
//
// The session ID is stored in a plain (non-sensitive) HTTP cookie.
// The cookie itself does NOT contain any session data.
type RedisStore struct {
	client redis.UniversalClient
	ttl    time.Duration
	prefix string
	opts   CookieOptions
}

// NewRedisStore creates a RedisStore backed by the given Redis client.
// ttl controls how long sessions live in Redis (renewed on every Save).
func NewRedisStore(client redis.UniversalClient, ttl time.Duration, options ...func(*CookieOptions)) *RedisStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	opts := defaultCookieOptions()
	for _, o := range options {
		o(&opts)
	}
	return &RedisStore{
		client: client,
		ttl:    ttl,
		prefix: "astra:session:",
		opts:   opts,
	}
}

// Load reads the session ID cookie and loads session data from Redis.
// Returns an empty session with a fresh ID if the cookie is absent or Redis has no entry.
func (s *RedisStore) Load(r *http.Request) (*Session, error) {
	sess := &Session{
		data:  make(map[string]any),
		store: s,
		name:  s.opts.Name,
		opts:  s.opts,
	}

	cookie, err := r.Cookie(s.opts.Name)
	if err != nil || cookie.Value == "" {
		sess.id = newSessionID()
		return sess, nil
	}

	sess.id = cookie.Value
	key := s.redisKey(sess.id)

	raw, err := s.client.Get(r.Context(), key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// No session in Redis — start fresh with same ID.
			return sess, nil
		}
		return nil, fmt.Errorf("session: Redis load error: %w", err)
	}

	data, err := unmarshalData(raw)
	if err != nil {
		// Corrupted data — start fresh.
		sess.id = newSessionID()
		return sess, nil
	}

	sess.data = data
	sess.loaded = true
	return sess, nil
}

// Save serialises the session data to Redis and sets/refreshes the ID cookie.
func (s *RedisStore) Save(w http.ResponseWriter, sess *Session) error {
	if sess.id == "" {
		sess.id = newSessionID()
	}

	payload, err := marshalData(sess.data)
	if err != nil {
		return fmt.Errorf("session: RedisStore.Save marshal: %w", err)
	}

	key := s.redisKey(sess.id)
	ctx := context.Background()
	if err := s.client.Set(ctx, key, payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("session: RedisStore.Save redis: %w", err)
	}

	setCookie(w, sess.name, sess.id, sess.opts)
	return nil
}

// Destroy deletes the session from Redis and clears the cookie.
func (s *RedisStore) Destroy(w http.ResponseWriter, sess *Session) error {
	if sess.id != "" {
		_ = s.client.Del(context.Background(), s.redisKey(sess.id))
	}
	clearCookie(w, sess.name, sess.opts.Path)
	return nil
}

// Regenerate issues a new session ID, migrates data in Redis, and updates the cookie.
func (s *RedisStore) Regenerate(w http.ResponseWriter, sess *Session) error {
	oldID := sess.id
	newID := newSessionID()

	// If we have an old ID, delete it from Redis
	if oldID != "" {
		ctx := context.Background()
		_ = s.client.Del(ctx, s.redisKey(oldID))
	}

	// Update session with new ID and mark dirty to ensure Save is called
	sess.id = newID
	sess.dirty = true

	// Force an immediate save to Redis and cookie update
	return s.Save(w, sess)
}

func (s *RedisStore) redisKey(id string) string {
	return s.prefix + id
}

// newSessionID generates a cryptographically random 128-bit hex session ID.
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("session: failed to generate session ID: " + err.Error())
	}
	return hex.EncodeToString(b)
}
