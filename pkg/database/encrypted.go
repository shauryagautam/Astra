package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/shauryagautam/Astra/pkg/engine/json"
	"golang.org/x/crypto/hkdf"
)

// Encrypted wraps a value with transparent crypto.
// Use this for sensitive PII like email, phone, or credentials.
type Encrypted[T any] struct {
	Val T
}

var encryptionKey []byte
var errNoKey = errors.New("orm: APP_KEY is not initialized; call orm.InitializeEncryption(key) during boot")

// InitializeEncryption derives the 32-byte AES key from the application key.
// This should be called during application startup (e.g., by the ORM provider).
func InitializeEncryption(appKey string) error {
	if appKey == "" {
		return fmt.Errorf("orm: cannot initialize encryption with an empty key")
	}

	// Derive 32-byte key using HKDF-SHA256
	kdf := hkdf.New(sha256.New, []byte(appKey), nil, []byte("astra-orm-encryption"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(kdf, key); err != nil {
		return fmt.Errorf("orm: failed to derive encryption key: %w", err)
	}

	encryptionKey = key
	return nil
}

// Scan implements sql.Scanner interface for the ORM
func (e *Encrypted[T]) Scan(src any) error {
	if src == nil {
		return nil
	}

	var encrypted string
	switch v := src.(type) {
	case string:
		encrypted = v
	case []byte:
		encrypted = string(v)
	default:
		return fmt.Errorf("cannot scan %T into Encrypted", src)
	}

	decrypted, err := decrypt(encrypted)
	if err != nil {
		return err
	}

	return json.UnmarshalString(decrypted, &e.Val)
}

// Value implements driver.Valuer interface (conceptually)
func (e Encrypted[T]) Value() (any, error) {
	data, err := json.MarshalString(e.Val)
	if err != nil {
		return nil, err
	}

	encrypted, err := encrypt(data)
	if err != nil {
		return nil, err
	}

	return encrypted, nil
}

func encrypt(plaintext string) (string, error) {
	if encryptionKey == nil {
		return "", errNoKey
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(ciphertext string) (string, error) {
	if encryptionKey == nil {
		return "", errNoKey
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
