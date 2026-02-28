package auth

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// ApiToken represents an opaque access token stored in the database.
type ApiToken struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"index"`
	Name      string    `gorm:"size:255"`
	Token     string    `gorm:"size:255;uniqueIndex"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DatabaseTokenStore implements the TokenStore interface using GORM.
type DatabaseTokenStore struct {
	db *gorm.DB
}

// NewDatabaseTokenStore creates a new database-backed token store.
func NewDatabaseTokenStore(db *gorm.DB) *DatabaseTokenStore {
	// Auto-migrate the table
	_ = db.AutoMigrate(&ApiToken{})

	return &DatabaseTokenStore{
		db: db,
	}
}

// Store saves a token associated with a user.
func (s *DatabaseTokenStore) Store(userID string, token string, name string, expiresAt time.Time) error {
	apiToken := &ApiToken{
		UserID:    userID,
		Token:     token,
		Name:      name,
		ExpiresAt: expiresAt,
	}

	return s.db.Create(apiToken).Error
}

// Find looks up a token and returns the associated user ID.
func (s *DatabaseTokenStore) Find(token string) (string, error) {
	var apiToken ApiToken
	err := s.db.Where("token = ?", token).First(&apiToken).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInvalidToken
		}
		return "", err
	}

	// Check expiration
	if time.Now().After(apiToken.ExpiresAt) {
		_ = s.Revoke(token)
		return "", ErrTokenExpired
	}

	return apiToken.UserID, nil
}

// Revoke deletes a specific token.
func (s *DatabaseTokenStore) Revoke(token string) error {
	return s.db.Where("token = ?", token).Delete(&ApiToken{}).Error
}

// RevokeAll deletes all tokens for a user.
func (s *DatabaseTokenStore) RevokeAll(userID string) error {
	return s.db.Where("user_id = ?", userID).Delete(&ApiToken{}).Error
}

var _ TokenStore = (*DatabaseTokenStore)(nil)
