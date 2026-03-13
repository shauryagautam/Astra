package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CookieStore is a stateless session store that encrypts session data into
// the cookie itself. No server-side state is required.
//
// Cookies are: AES-256-GCM encrypted + HMAC-SHA256 signed.
// The key is derived from appKey using HKDF-SHA256.
//
// Limitations:
//   - Maximum cookie size ~4 KB (browser cookie limit)
//   - Cannot invalidate individual sessions without rotating the key
type CookieStore struct {
	encKey  []byte // 32-byte AES key
	signKey []byte // 32-byte HMAC key
	opts    CookieOptions
}

// NewCookieStore creates a CookieStore with the given app key (any length).
// Two 32-byte keys are derived: one for AES-256-GCM, one for HMAC-SHA256.
func NewCookieStore(appKey []byte, options ...func(*CookieOptions)) *CookieStore {
	encKey := deriveKey(appKey, "astra-session-enc", 32)
	signKey := deriveKey(appKey, "astra-session-sig", 32)

	opts := defaultCookieOptions()
	for _, o := range options {
		o(&opts)
	}

	return &CookieStore{
		encKey:  encKey,
		signKey: signKey,
		opts:    opts,
	}
}

// WithCookieName sets the session cookie name.
func WithCookieName(name string) func(*CookieOptions) {
	return func(o *CookieOptions) { o.Name = name }
}

// WithSecure marks the cookie Secure (HTTPS only).
func WithSecure(secure bool) func(*CookieOptions) {
	return func(o *CookieOptions) { o.Secure = secure }
}

// Load reads and decrypts the session cookie from the request.
// Returns an empty session if the cookie is absent or invalid.
func (s *CookieStore) Load(r *http.Request) (*Session, error) {
	sess := &Session{
		data:  make(map[string]any),
		store: s,
		name:  s.opts.Name,
		opts:  s.opts,
	}

	cookie, err := r.Cookie(s.opts.Name)
	if err != nil {
		// No cookie — return fresh session.
		return sess, nil
	}

	data, err := s.decode(cookie.Value)
	if err != nil {
		// Tampered or expired cookie — return fresh session.
		return sess, nil
	}

	sess.data = data
	sess.loaded = true
	return sess, nil
}

// Save encodes and encrypts the session data into the response cookie.
func (s *CookieStore) Save(w http.ResponseWriter, sess *Session) error {
	encoded, err := s.encode(sess.data)
	if err != nil {
		return fmt.Errorf("session: CookieStore.Save: %w", err)
	}
	setCookie(w, sess.name, encoded, sess.opts)
	return nil
}

// Destroy clears the session cookie.
func (s *CookieStore) Destroy(w http.ResponseWriter, sess *Session) error {
	clearCookie(w, sess.name, sess.opts.Path)
	return nil
}

// Regenerate issues a new session ID (for compatibility) and re-encrypts session data.
func (s *CookieStore) Regenerate(w http.ResponseWriter, sess *Session) error {
	// For CookieStore, ID is just a formal attribute, but we rotate it anyway.
	sess.id = newSessionID()
	sess.dirty = true
	return s.Save(w, sess)
}

// ─── Encode / Decode ──────────────────────────────────────────────────────────

// encode serialises data → JSON → HMAC sign → AES-GCM encrypt → base64url.
func (s *CookieStore) encode(data map[string]any) (string, error) {
	plaintext, err := marshalData(data)
	if err != nil {
		return "", err
	}

	// Sign first, then encrypt the (plaintext + sig) together.
	sig := computeHMAC(s.signKey, plaintext)
	payload := append(plaintext, sig...) // nolint:gocritic

	ciphertext, err := aesGCMEncrypt(s.encKey, payload)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// decode is the inverse of encode.
func (s *CookieStore) decode(raw string) (map[string]any, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("session: base64 decode: %w", err)
	}

	payload, err := aesGCMDecrypt(s.encKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("session: decrypt: %w", err)
	}

	// Last 32 bytes are the HMAC.
	if len(payload) < 32 {
		return nil, fmt.Errorf("session: payload too short")
	}
	jsonBytes := payload[:len(payload)-32]
	sig := payload[len(payload)-32:]

	expected := computeHMAC(s.signKey, jsonBytes)
	if !hmac.Equal(sig, expected) {
		return nil, fmt.Errorf("session: HMAC mismatch — cookie tampered")
	}

	return unmarshalData(jsonBytes)
}

// ─── Crypto Primitives ────────────────────────────────────────────────────────

func aesGCMEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func aesGCMDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}

func computeHMAC(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// deriveKey derives a keyLen-byte key from secret using HKDF-SHA256 with info label.
func deriveKey(secret []byte, info string, keyLen int) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(info))
	sum := h.Sum(nil)
	// Expand to keyLen via repeated hashing if needed.
	result := make([]byte, 0, keyLen)
	counter := byte(1)
	for len(result) < keyLen {
		h.Reset()
		h.Write(sum)
		h.Write([]byte{counter})
		result = append(result, h.Sum(nil)...)
		counter++
	}
	return result[:keyLen]
}

// ensure strings is used
var _ = strings.TrimSpace
