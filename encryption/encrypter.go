package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	// DefaultEncrypter is a global encrypter used by transparently encrypted fields.
	DefaultEncrypter *Encrypter
)

// InitDefaultEncrypter initializes the global encrypter.
func InitDefaultEncrypter(key string) error {
	var err error
	DefaultEncrypter, err = NewEncrypter(key)
	return err
}

// Encrypter providing AES-256-GCM encryption.
type Encrypter struct {
	key []byte
}

// NewEncrypter creates a new encrypter with the given 32-byte key.
func NewEncrypter(key string) (*Encrypter, error) {
	k := []byte(key)
	if len(k) != 32 {
		return nil, errors.New("encryption: key must be exactly 32 bytes for AES-256")
	}
	return &Encrypter{key: k}, nil
}

// Encrypt encrypts the given plaintext and returns a base64 encoded string.
func (e *Encrypter) Encrypt(plaintext string) (string, error) {
	if e == nil || e.key == nil {
		return "", errors.New("encryption: encrypter is not initialized")
	}
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("encryption: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryption: failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encryption: failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts the given base64 encoded ciphertext.
func (e *Encrypter) Decrypt(encodedCiphertext string) (string, error) {
	if e == nil || e.key == nil {
		return "", errors.New("encryption: encrypter is not initialized")
	}
	if encodedCiphertext == "" {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", fmt.Errorf("encryption: failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("encryption: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryption: failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("encryption: ciphertext too short")
	}

	nonce, encryptedMsg := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedMsg, nil)
	if err != nil {
		return "", fmt.Errorf("encryption: decryption failed: %w", err)
	}

	return string(plaintext), nil
}
