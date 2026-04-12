package keys

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// APIKey represents an API key
type APIKey struct {
	ID          string                 `json:"id"`
	KeyID       string                 `json:"key_id"`     // Public identifier
	KeySecret   string                 `json:"key_secret"` // Full key (encrypted)
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	UserID      string                 `json:"user_id"`
	Scopes      []string               `json:"scopes"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExpiresAt   *time.Time             `json:"expires_at"`
	LastUsed    *time.Time             `json:"last_used"`
	CreatedAt   time.Time              `json:"created_at"`
	RevokedAt   *time.Time             `json:"revoked_at"`
	Active      bool                   `json:"active"`
	RateLimit   *RateLimit             `json:"rate_limit"`
}

// RateLimit represents rate limiting configuration
type RateLimit struct {
	RequestsPerSecond int `json:"requests_per_second"`
	RequestsPerHour   int `json:"requests_per_hour"`
	RequestsPerDay    int `json:"requests_per_day"`
}

// Usage represents API key usage statistics
type Usage struct {
	KeyID           string    `json:"key_id"`
	Requests        int       `json:"requests"`
	LastRequest     time.Time `json:"last_request"`
	RequestsPerDay  int       `json:"requests_per_day"`
	RequestsPerHour int       `json:"requests_per_hour"`
}

// Storage interface for API key persistence
type Storage interface {
	// API Keys
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKey(ctx context.Context, keyID string) (*APIKey, error)
	GetAPIKeyBySecret(ctx context.Context, secret string) (*APIKey, error)
	ListAPIKeys(ctx context.Context, userID string) ([]*APIKey, error)
	ListAllAPIKeys(ctx context.Context) ([]*APIKey, error)
	UpdateAPIKey(ctx context.Context, key *APIKey) error
	RevokeAPIKey(ctx context.Context, keyID string) error
	DeleteAPIKey(ctx context.Context, keyID string) error

	// Usage tracking
	RecordUsage(ctx context.Context, keyID string, endpoint string) error
	GetUsage(ctx context.Context, keyID string) (*Usage, error)
	GetUsageStats(ctx context.Context, timeframe string) ([]*Usage, error)
}

// Manager manages API keys
type Manager struct {
	storage   Storage
	encryptor Encryptor
	logger    Logger
	config    Config
}

// Encryptor interface for key encryption/decryption
type Encryptor interface {
	Encrypt(data string) (string, error)
	Decrypt(data string) (string, error)
}

// Logger interface for logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// Config represents API key configuration
type Config struct {
	KeyLength        int           `json:"key_length"`
	DefaultExpiresIn time.Duration `json:"default_expires_in"`
	MaxKeysPerUser   int           `json:"max_keys_per_user"`
	RequireApproval  bool          `json:"require_approval"`
	DefaultScopes    []string      `json:"default_scopes"`
}

// NewManager creates a new API key manager
func NewManager(storage Storage, encryptor Encryptor, logger Logger, config Config) *Manager {
	return &Manager{
		storage:   storage,
		encryptor: encryptor,
		logger:    logger,
		config:    config,
	}
}

// GenerateAPIKey generates a new API key
func (m *Manager) GenerateAPIKey(ctx context.Context, userID, name, description string, scopes []string, expiresAt *time.Time) (*APIKey, error) {
	// Check user's key limit
	userKeys, err := m.storage.ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check user keys: %w", err)
	}

	if len(userKeys) >= m.config.MaxKeysPerUser {
		return nil, fmt.Errorf("user has reached maximum key limit (%d)", m.config.MaxKeysPerUser)
	}

	// Generate key components
	keyID := m.generateKeyID()
	keySecret := m.generateKeySecret()

	// Encrypt the full key
	encryptedSecret, err := m.encryptor.Encrypt(keySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt key: %w", err)
	}

	// Set default scopes if none provided
	if len(scopes) == 0 {
		scopes = m.config.DefaultScopes
	}

	// Set default expiration if none provided
	if expiresAt == nil {
		exp := time.Now().Add(m.config.DefaultExpiresIn)
		expiresAt = &exp
	}

	// Create API key
	apiKey := &APIKey{
		ID:          m.generateID(),
		KeyID:       keyID,
		KeySecret:   encryptedSecret,
		Name:        name,
		Description: description,
		UserID:      userID,
		Scopes:      scopes,
		Metadata:    make(map[string]interface{}),
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
		Active:      true,
	}

	// Store the key
	if err := m.storage.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("failed to store API key: %w", err)
	}

	m.logger.Info("API key generated", "key_id", keyID, "user_id", userID, "name", name)

	// Return the key with the unencrypted secret for one-time display
	result := *apiKey
	result.KeySecret = keySecret
	return &result, nil
}

// ValidateAPIKey validates an API key
func (m *Manager) ValidateAPIKey(ctx context.Context, keySecret string) (*APIKey, error) {
	// Encrypt the provided secret to match stored format
	encryptedSecret, err := m.encryptor.Encrypt(keySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt key for validation: %w", err)
	}

	// Look up the key
	apiKey, err := m.storage.GetAPIKeyBySecret(ctx, encryptedSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	// Check if key is active
	if !apiKey.Active {
		return nil, fmt.Errorf("API key is revoked")
	}

	// Check if key has expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Update last used timestamp
	now := time.Now()
	apiKey.LastUsed = &now
	if err := m.storage.UpdateAPIKey(ctx, apiKey); err != nil {
		m.logger.Error("Failed to update last used timestamp", "error", err, "key_id", apiKey.KeyID)
	}

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (m *Manager) RevokeAPIKey(ctx context.Context, keyID string) error {
	apiKey, err := m.storage.GetAPIKey(ctx, keyID)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	now := time.Now()
	apiKey.RevokedAt = &now
	apiKey.Active = false

	if err := m.storage.UpdateAPIKey(ctx, apiKey); err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	m.logger.Info("API key revoked", "key_id", keyID, "user_id", apiKey.UserID)
	return nil
}

// DeleteAPIKey permanently deletes an API key
func (m *Manager) DeleteAPIKey(ctx context.Context, keyID string) error {
	if err := m.storage.DeleteAPIKey(ctx, keyID); err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	m.logger.Info("API key deleted", "key_id", keyID)
	return nil
}

// UpdateAPIKey updates an API key
func (m *Manager) UpdateAPIKey(ctx context.Context, keyID string, updates map[string]interface{}) error {
	apiKey, err := m.storage.GetAPIKey(ctx, keyID)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		apiKey.Name = name
	}
	if description, ok := updates["description"].(string); ok {
		apiKey.Description = description
	}
	if scopes, ok := updates["scopes"].([]string); ok {
		apiKey.Scopes = scopes
	}
	if expiresAt, ok := updates["expires_at"].(*time.Time); ok {
		apiKey.ExpiresAt = expiresAt
	}
	if active, ok := updates["active"].(bool); ok {
		apiKey.Active = active
		if !active {
			now := time.Now()
			apiKey.RevokedAt = &now
		}
	}

	if err := m.storage.UpdateAPIKey(ctx, apiKey); err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	m.logger.Info("API key updated", "key_id", keyID, "user_id", apiKey.UserID)
	return nil
}

// GetUserAPIKeys returns all API keys for a user
func (m *Manager) GetUserAPIKeys(ctx context.Context, userID string) ([]*APIKey, error) {
	keys, err := m.storage.ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user API keys: %w", err)
	}

	// Remove secrets from response
	for _, key := range keys {
		key.KeySecret = ""
	}

	return keys, nil
}

// GetAPIKey returns an API key by ID (without secret)
func (m *Manager) GetAPIKey(ctx context.Context, keyID string) (*APIKey, error) {
	apiKey, err := m.storage.GetAPIKey(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Remove secret from response
	apiKey.KeySecret = ""
	return apiKey, nil
}

// CheckScope checks if an API key has the required scope
func (m *Manager) CheckScope(apiKey *APIKey, requiredScope string) bool {
	for _, scope := range apiKey.Scopes {
		if scope == "*" || scope == requiredScope {
			return true
		}
	}
	return false
}

// RecordUsage records API key usage
func (m *Manager) RecordUsage(ctx context.Context, keyID, endpoint string) error {
	if err := m.storage.RecordUsage(ctx, keyID, endpoint); err != nil {
		m.logger.Error("Failed to record usage", "error", err, "key_id", keyID)
		return err
	}
	return nil
}

// GetUsage returns usage statistics for an API key
func (m *Manager) GetUsage(ctx context.Context, keyID string) (*Usage, error) {
	usage, err := m.storage.GetUsage(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage: %w", err)
	}
	return usage, nil
}

// GetUsageStats returns overall usage statistics
func (m *Manager) GetUsageStats(ctx context.Context, timeframe string) ([]*Usage, error) {
	stats, err := m.storage.GetUsageStats(ctx, timeframe)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}
	return stats, nil
}

// generateKeyID generates a public key identifier
func (m *Manager) generateKeyID() string {
	return fmt.Sprintf("ak_%s_%s",
		time.Now().Format("20060102"),
		m.generateRandomString(8))
}

// generateKeySecret generates a full API key secret
func (m *Manager) generateKeySecret() string {
	return fmt.Sprintf("ask_%s_%s",
		time.Now().Format("20060102150405"),
		m.generateRandomString(32))
}

// generateID generates a unique ID
func (m *Manager) generateID() string {
	return m.generateRandomString(16)
}

// generateRandomString generates a random string
func (m *Manager) generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate random bytes: %v", err))
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// SimpleEncryptor provides simple encryption for API keys
type SimpleEncryptor struct {
	secret string
}

// NewSimpleEncryptor creates a new simple encryptor
func NewSimpleEncryptor(secret string) *SimpleEncryptor {
	return &SimpleEncryptor{secret: secret}
}

// Encrypt encrypts data using simple XOR encryption
func (e *SimpleEncryptor) Encrypt(data string) (string, error) {
	if e.secret == "" {
		return data, nil
	}

	// Simple XOR encryption (for demonstration only)
	result := make([]byte, len(data))
	secretBytes := []byte(e.secret)

	for i, b := range []byte(data) {
		result[i] = b ^ secretBytes[i%len(secretBytes)]
	}

	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt decrypts data using simple XOR encryption
func (e *SimpleEncryptor) Decrypt(data string) (string, error) {
	if e.secret == "" {
		return data, nil
	}

	// Decode base64
	encrypted, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	// Simple XOR decryption
	result := make([]byte, len(encrypted))
	secretBytes := []byte(e.secret)

	for i, b := range encrypted {
		result[i] = b ^ secretBytes[i%len(secretBytes)]
	}

	return string(result), nil
}

// ScopeValidator validates API key scopes
type ScopeValidator struct {
	validScopes map[string]bool
}

// NewScopeValidator creates a new scope validator
func NewScopeValidator(scopes []string) *ScopeValidator {
	validScopes := make(map[string]bool)
	for _, scope := range scopes {
		validScopes[scope] = true
	}
	return &ScopeValidator{validScopes: validScopes}
}

// ValidateScope validates a scope
func (sv *ScopeValidator) ValidateScope(scope string) bool {
	return sv.validScopes[scope] || scope == "*"
}

// ValidateScopes validates multiple scopes
func (sv *ScopeValidator) ValidateScopes(scopes []string) error {
	for _, scope := range scopes {
		if !sv.ValidateScope(scope) {
			return fmt.Errorf("invalid scope: %s", scope)
		}
	}
	return nil
}

// RateLimiter provides rate limiting for API keys
type RateLimiter struct {
	storage map[string]*RateLimitData
}

// RateLimitData tracks rate limiting data
type RateLimitData struct {
	RequestsPerSecond int
	RequestsPerHour   int
	RequestsPerDay    int
	LastRequest       time.Time
	SecondCount       int
	HourCount         int
	DayCount          int
	CurrentSecond     int
	CurrentHour       int
	CurrentDay        int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		storage: make(map[string]*RateLimitData),
	}
}

// CheckRateLimit checks if a request is within rate limits
func (rl *RateLimiter) CheckRateLimit(keyID string, rateLimit *RateLimit) bool {
	if rateLimit == nil {
		return true
	}

	now := time.Now()
	data, exists := rl.storage[keyID]
	if !exists {
		data = &RateLimitData{
			LastRequest: now,
		}
		rl.storage[keyID] = data
	}

	// Reset counters if needed
	if now.Day() != data.CurrentDay {
		data.DayCount = 0
		data.CurrentDay = now.Day()
	}
	if now.Hour() != data.CurrentHour {
		data.HourCount = 0
		data.CurrentHour = now.Hour()
	}
	if now.Second() != data.CurrentSecond {
		data.SecondCount = 0
		data.CurrentSecond = now.Second()
	}

	// Check limits
	if rateLimit.RequestsPerSecond > 0 && data.SecondCount >= rateLimit.RequestsPerSecond {
		return false
	}
	if rateLimit.RequestsPerHour > 0 && data.HourCount >= rateLimit.RequestsPerHour {
		return false
	}
	if rateLimit.RequestsPerDay > 0 && data.DayCount >= rateLimit.RequestsPerDay {
		return false
	}

	// Increment counters
	data.SecondCount++
	data.HourCount++
	data.DayCount++
	data.LastRequest = now

	return true
}

// Cleanup removes old rate limit data
func (rl *RateLimiter) Cleanup() {
	// Remove entries older than 24 hours
	cutoff := time.Now().Add(-24 * time.Hour)
	for keyID, data := range rl.storage {
		if data.LastRequest.Before(cutoff) {
			delete(rl.storage, keyID)
		}
	}
}
