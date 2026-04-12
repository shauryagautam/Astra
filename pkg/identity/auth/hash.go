package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Hasher defines the interface for password hashing.
type Hasher interface {
	Make(plain string) (string, error)
	Check(plain, hash string) bool
	NeedsRehash(hash string) bool
}

// Argon2idHasher configures the argon2id algorithm.
type Argon2idHasher struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// NewArgon2idHasher returns a Hasher with OWASP recommended defaults for Argon2id.
// Time: 3 iterations
// Memory: 64 MB (64 * 1024)
// Parallelism: 4 threads
func NewArgon2idHasher() *Argon2idHasher {
	return &Argon2idHasher{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// Make hashes a password using argon2id and returns the PHC string format.
func (h *Argon2idHasher) Make(plain string) (string, error) {
	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(plain), salt, h.Iterations, h.Memory, h.Parallelism, h.KeyLength)

	// Format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.Memory, h.Iterations, h.Parallelism, b64Salt, b64Hash), nil
}

// Check verifies a plain password against a PHC formatted argon2id hash.
func (h *Argon2idHasher) Check(plain, hashStr string) bool {
	p, err := h.decodeHash(hashStr)
	if err != nil {
		return false
	}

	otherHash := argon2.IDKey([]byte(plain), p.salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	return subtle.ConstantTimeCompare(p.hash, otherHash) == 1
}

// NeedsRehash returns true if the hash format or parameters don't match the current configuration.
func (h *Argon2idHasher) NeedsRehash(hashStr string) bool {
	p, err := h.decodeHash(hashStr)
	if err != nil {
		return true // Invalid format, definitely needs rehash
	}
	return p.memory != h.Memory || p.iterations != h.Iterations || p.parallelism != h.Parallelism
}

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	hash        []byte
	keyLength   uint32
}

func (h *Argon2idHasher) decodeHash(hashStr string) (*argon2Params, error) {
	parts := strings.Split(hashStr, "$")
	if len(parts) != 6 {
		return nil, errors.New("argon2id: invalid hash format")
	}

	if parts[1] != "argon2id" {
		return nil, errors.New("argon2id: incompatible hash algorithm")
	}

	var version int
	_, err := fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil || version != argon2.Version {
		return nil, errors.New("argon2id: incompatible or unparseable version")
	}

	p := &argon2Params{}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, errors.New("argon2id: unparseable parameters")
	}

	p.salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, errors.New("argon2id: unparseable salt")
	}

	p.hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, errors.New("argon2id: unparseable hash")
	}
	p.keyLength = uint32(len(p.hash))

	return p, nil
}
