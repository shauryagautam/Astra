package auth

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDatabaseTokenStore(t *testing.T) {
	// Initialize in-memory SQLite for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	store := NewDatabaseTokenStore(db)
	userID := "user_1"
	token := "secret_oat_token"
	name := "mobile_app"
	expiry := time.Now().Add(1 * time.Hour)

	// 1. Store token
	err = store.Store(userID, token, name, expiry)
	if err != nil {
		t.Errorf("failed to store token: %v", err)
	}

	// 2. Find token
	foundUserID, err := store.Find(token)
	if err != nil {
		t.Errorf("failed to find token: %v", err)
	}
	if foundUserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, foundUserID)
	}

	// 3. Revoke token
	err = store.Revoke(token)
	if err != nil {
		t.Errorf("failed to revoke token: %v", err)
	}

	// 4. Find revoked token should fail
	_, err = store.Find(token)
	if err == nil {
		t.Errorf("expected error finding revoked token, got nil")
	}

	// 5. RevokeAll
	_ = store.Store(userID, "token_1", "app_1", expiry)
	_ = store.Store(userID, "token_2", "app_2", expiry)
	err = store.RevokeAll(userID)
	if err != nil {
		t.Errorf("failed to revoke all tokens: %v", err)
	}

	_, err = store.Find("token_1")
	if err == nil {
		t.Errorf("expected error finding revoked all (token_1), got nil")
	}
}

func TestDatabaseTokenStoreExpiration(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	store := NewDatabaseTokenStore(db)

	token := "expired_token"
	expiry := time.Now().Add(-1 * time.Hour)
	_ = store.Store("user_1", token, "app", expiry)

	_, err := store.Find(token)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}
