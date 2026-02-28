// Package hash provides the Hash module — a unified hashing API
// with pluggable drivers (Argon2id, Bcrypt).
// Mirrors AdonisJS's @adonisjs/hash module.
//
// Usage:
//
//	hasher := hash.NewManager("argon2", hash.DefaultArgon2Config(), hash.DefaultBcryptConfig())
//	hashed, _ := hasher.Make("my-password")
//	match, _ := hasher.Verify(hashed, "my-password") // true
package hash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// ══════════════════════════════════════════════════════════════════════
// Argon2 Driver
// ══════════════════════════════════════════════════════════════════════

// Argon2Config holds Argon2id configuration.
type Argon2Config struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2Config returns default Argon2 settings.
func DefaultArgon2Config() Argon2Config {
	return Argon2Config{
		Memory:      65536, // 64MB
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}
}

// Argon2Driver hashes using Argon2id.
type Argon2Driver struct {
	config Argon2Config
}

// NewArgon2Driver creates a new Argon2 driver.
func NewArgon2Driver(config Argon2Config) *Argon2Driver {
	return &Argon2Driver{config: config}
}

// Make hashes the given value using Argon2id.
func (d *Argon2Driver) Make(value string) (string, error) {
	salt := make([]byte, d.config.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(value),
		salt,
		d.config.Iterations,
		d.config.Memory,
		d.config.Parallelism,
		d.config.KeyLength,
	)

	// Encode as: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		d.config.Memory,
		d.config.Iterations,
		d.config.Parallelism,
		b64Salt,
		b64Hash,
	)

	return encoded, nil
}

// Verify checks if a plain value matches an Argon2id hash.
func (d *Argon2Driver) Verify(hashedValue string, plainValue string) (bool, error) {
	parts := strings.Split(hashedValue, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid argon2id hash format")
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return false, fmt.Errorf("failed to parse argon2id params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	keyLength := uint32(len(expectedHash))
	computedHash := argon2.IDKey([]byte(plainValue), salt, iterations, memory, parallelism, keyLength)

	return subtle.ConstantTimeCompare(expectedHash, computedHash) == 1, nil
}

// NeedsRehash checks if the hash was created with different parameters.
func (d *Argon2Driver) NeedsRehash(hashedValue string) bool {
	parts := strings.Split(hashedValue, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return true
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)

	return memory != d.config.Memory ||
		iterations != d.config.Iterations ||
		parallelism != d.config.Parallelism
}

// ══════════════════════════════════════════════════════════════════════
// Bcrypt Driver
// ══════════════════════════════════════════════════════════════════════

// BcryptConfig holds Bcrypt configuration.
type BcryptConfig struct {
	Rounds int
}

// DefaultBcryptConfig returns default Bcrypt settings.
func DefaultBcryptConfig() BcryptConfig {
	return BcryptConfig{
		Rounds: 10,
	}
}

// BcryptDriver hashes using Bcrypt.
type BcryptDriver struct {
	config BcryptConfig
}

// NewBcryptDriver creates a new Bcrypt driver.
func NewBcryptDriver(config BcryptConfig) *BcryptDriver {
	return &BcryptDriver{config: config}
}

// Make hashes the given value using Bcrypt.
func (d *BcryptDriver) Make(value string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(value), d.config.Rounds)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash failed: %w", err)
	}
	return string(hash), nil
}

// Verify checks if a plain value matches a Bcrypt hash.
func (d *BcryptDriver) Verify(hashedValue string, plainValue string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hashedValue), []byte(plainValue))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// NeedsRehash checks if the cost factor differs.
func (d *BcryptDriver) NeedsRehash(hashedValue string) bool {
	cost, err := bcrypt.Cost([]byte(hashedValue))
	if err != nil {
		return true
	}
	return cost != d.config.Rounds
}

// ══════════════════════════════════════════════════════════════════════
// Hash Manager
// Mirrors AdonisJS's Hash module: Hash.make(), Hash.verify(), Hash.use()
// ══════════════════════════════════════════════════════════════════════

// Driver is the interface all hash drivers implement.
type Driver interface {
	Make(value string) (string, error)
	Verify(hashedValue string, plainValue string) (bool, error)
	NeedsRehash(hashedValue string) bool
}

// Manager is the Hash manager that delegates to configured drivers.
type Manager struct {
	defaultDriver string
	drivers       map[string]Driver
}

// NewManager creates a new Hash manager.
func NewManager(defaultDriver string, argon2Config Argon2Config, bcryptConfig BcryptConfig) *Manager {
	return &Manager{
		defaultDriver: defaultDriver,
		drivers: map[string]Driver{
			"argon2": NewArgon2Driver(argon2Config),
			"bcrypt": NewBcryptDriver(bcryptConfig),
		},
	}
}

// Make hashes a value using the default driver.
func (m *Manager) Make(value string) (string, error) {
	return m.Use(m.defaultDriver).Make(value)
}

// Verify checks if a plain value matches a hash using the default driver.
func (m *Manager) Verify(hashedValue string, plainValue string) (bool, error) {
	// Auto-detect driver from hash format
	if strings.HasPrefix(hashedValue, "$argon2id$") {
		return m.Use("argon2").Verify(hashedValue, plainValue)
	}
	if strings.HasPrefix(hashedValue, "$2a$") || strings.HasPrefix(hashedValue, "$2b$") {
		return m.Use("bcrypt").Verify(hashedValue, plainValue)
	}
	return m.Use(m.defaultDriver).Verify(hashedValue, plainValue)
}

// NeedsRehash checks if the hash needs rehashing.
func (m *Manager) NeedsRehash(hashedValue string) bool {
	if strings.HasPrefix(hashedValue, "$argon2id$") {
		return m.Use("argon2").NeedsRehash(hashedValue)
	}
	if strings.HasPrefix(hashedValue, "$2a$") || strings.HasPrefix(hashedValue, "$2b$") {
		return m.Use("bcrypt").NeedsRehash(hashedValue)
	}
	return m.Use(m.defaultDriver).NeedsRehash(hashedValue)
}

// Use returns a specific driver by name.
func (m *Manager) Use(driver string) Driver {
	if d, ok := m.drivers[driver]; ok {
		return d
	}
	panic(fmt.Sprintf("Hash driver '%s' not found. Available: argon2, bcrypt", driver))
}
