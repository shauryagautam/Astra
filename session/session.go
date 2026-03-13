package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultCookieName = "astra_session"
	defaultMaxAge     = 24 * time.Hour
)

// Session holds the session data for one request and knows how to
// persist itself back to the HTTP response.
type Session struct {
	id     string
	data   map[string]any
	store  Store
	name   string
	opts   CookieOptions
	loaded bool
	dirty  bool
}

// CookieOptions controls the session cookie attributes.
type CookieOptions struct {
	Name     string
	Path     string
	Domain   string
	MaxAge   time.Duration
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
}

func defaultCookieOptions() CookieOptions {
	return CookieOptions{
		Name:     defaultCookieName,
		Path:     "/",
		MaxAge:   defaultMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// Get retrieves a value by key. Returns nil if not present.
func (s *Session) Get(key string) any {
	return s.data[key]
}

// GetString retrieves a string value or empty string.
func (s *Session) GetString(key string) string {
	if v, ok := s.data[key].(string); ok {
		return v
	}
	return ""
}

// GetInt retrieves an int value (stored as float64 from JSON) or 0.
func (s *Session) GetInt(key string) int {
	switch v := s.data[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return 0
}

// Set stores a value by key. Marks the session as dirty.
func (s *Session) Set(key string, value any) {
	s.data[key] = value
	s.dirty = true
}

// Delete removes a key from the session.
func (s *Session) Delete(key string) {
	delete(s.data, key)
	s.dirty = true
}

// Clear removes all session data.
func (s *Session) Clear() {
	s.data = make(map[string]any)
	s.dirty = true
}

// Has returns true if the key exists in the session.
func (s *Session) Has(key string) bool {
	_, ok := s.data[key]
	return ok
}

// Flash returns a flash value and removes it from the session immediately.
func (s *Session) Flash(key string) any {
	val := s.data[key]
	if val != nil {
		delete(s.data, key)
		s.dirty = true
	}
	return val
}

// ID returns the session ID (meaningful for server-side stores; empty for CookieStore).
func (s *Session) ID() string { return s.id }

// Save persists the session to the response. Must be called before the
// response body is written. For server-side stores this writes the ID cookie.
// For CookieStore this writes the encrypted data cookie.
func (s *Session) Save(w http.ResponseWriter) error {
	if !s.dirty {
		return nil
	}
	return s.store.Save(w, s)
}

// ─── Store Interface ──────────────────────────────────────────────────────────

// Store is the low-level backend for session persistence.
type Store interface {
	// Load reads a session from the request. Returns an empty session if none found.
	Load(r *http.Request) (*Session, error)
	// Save writes the session to the response (sets cookies, updates Redis, etc.)
	Save(w http.ResponseWriter, s *Session) error
	// Destroy invalidates the session and clears the cookie.
	Destroy(w http.ResponseWriter, s *Session) error
	// Regenerate issues a new session ID while preserving data.
	Regenerate(w http.ResponseWriter, s *Session) error
}

// Regenerate issues a new session ID for the current session while preserving
// its data. This should be called after login or privilege escalation.
func (s *Session) Regenerate(w http.ResponseWriter) error {
	return s.store.Regenerate(w, s)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// marshalData serializes session data to JSON bytes.
func marshalData(data map[string]any) ([]byte, error) {
	return json.Marshal(data)
}

// unmarshalData deserializes JSON bytes into session data map.
func unmarshalData(b []byte) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// setCookie writes an HTTP cookie to the response.
func setCookie(w http.ResponseWriter, name, value string, opts CookieOptions) {
	maxAgeSecs := int(opts.MaxAge.Seconds())
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     opts.Path,
		Domain:   opts.Domain,
		MaxAge:   maxAgeSecs,
		Secure:   opts.Secure,
		HttpOnly: opts.HttpOnly,
		SameSite: opts.SameSite,
	})
}

// clearCookie immediately expires a cookie.
func clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Value:   "",
		Path:    path,
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
}

// ensure packages are used
var _ = fmt.Sprintf
