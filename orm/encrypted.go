package orm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/astraframework/astra/json"
	"golang.org/x/crypto/hkdf"
)

// Encrypted wraps a value with transparent encryption.
// Use this for sensitive PII like email, phone, or credentials.
type Encrypted[T any] struct {
	Val T
}

var encryptionKey []byte

func init() {
	appKey := os.Getenv("APP_KEY")
	if appKey == "" {
		appKey = "default-insecure-key-change-in-production"
	}

	// Derive 32-byte key using HKDF-SHA256
	kdf := hkdf.New(sha256.New, []byte(appKey), nil, []byte("astra-orm-encryption"))
	encryptionKey = make([]byte, 32)
	if _, err := io.ReadFull(kdf, encryptionKey); err != nil {
		panic(fmt.Sprintf("failed to derive encryption key: %v", err))
	}
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
