package crypto

import (
	"testing"
)

func TestEncryption(t *testing.T) {
	key := "01234567890123456789012345678901" // 32 bytes
	e, err := NewEncrypter(key)
	if err != nil {
		t.Fatalf("Failed to create encrypter: %v", err)
	}

	original := "Hello, Astra Framework!"
	encrypted, err := e.Encrypt(original)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if encrypted == original {
		t.Error("Encrypted text should not equal original text")
	}

	decrypted, err := e.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted != original {
		t.Errorf("Expected decrypted text to be '%s', got '%s'", original, decrypted)
	}
}

func TestEncryptionWithShortKey(t *testing.T) {
	_, err := NewEncrypter("short")
	if err == nil {
		t.Error("Expected error with short key, got nil")
	}
}
