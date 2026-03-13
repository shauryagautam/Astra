package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// JWTManager handles issuing and verifying JWT tokens.
type JWTManager struct {
	config      config.AuthConfig
	redisClient *redis.Client
	keys        map[string][]byte
	activeKeyID string
}

// NewJWTManager creates a new JWTManager.
func NewJWTManager(cfg config.AuthConfig, redisClient *redis.Client) *JWTManager {
	mgr := &JWTManager{
		config:      cfg,
		redisClient: redisClient,
		keys:        make(map[string][]byte),
	}

	mgr.loadSecrets(cfg.JWTSecret)
	return mgr
}

func (m *JWTManager) loadSecrets(secretStr string) {
	secrets := strings.Split(secretStr, ",")
	for i, s := range secrets {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		var kid, secret string
		if parts := strings.SplitN(s, ":", 2); len(parts) == 2 {
			kid = parts[0]
			secret = parts[1]
		} else {
			hash := sha256.Sum256([]byte(s))
			kid = fmt.Sprintf("%x", hash)[:8]
			secret = s
		}

		m.keys[kid] = []byte(secret)
		if i == 0 {
			m.activeKeyID = kid
		}
	}

	if len(m.keys) == 0 {
		m.keys["default"] = []byte(m.config.JWTSecret)
		m.activeKeyID = "default"
	}
}

// Validate checks if the current key configuration is valid and sufficiently strong.
func (m *JWTManager) Validate() error {
	if len(m.keys) == 0 {
		return fmt.Errorf("jwt: no keys configured")
	}
	for kid, key := range m.keys {
		if len(key) < 32 {
			return fmt.Errorf("jwt: secret for key %s is too short (min 32 bytes for HS256)", kid)
		}
	}
	if _, ok := m.keys[m.activeKeyID]; !ok {
		return fmt.Errorf("jwt: active key id %s not found in keys map", m.activeKeyID)
	}
	return nil
}

// TokenPair represents a pair of access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// IssueTokenPair generates a new access and refresh token pair for a user.
func (m *JWTManager) IssueTokenPair(ctx context.Context, userID string, customClaims map[string]any) (*TokenPair, error) {
	// Access Token
	accessClaims := jwt.MapClaims{
		"sub": userID,
		"iss": m.config.JWTIssuer,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(m.config.AccessTokenExpiry).Unix(),
		"jti": uuid.New().String(),
	}
	for k, v := range customClaims {
		accessClaims[k] = v
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken.Header["kid"] = m.activeKeyID
	accessString, err := accessToken.SignedString(m.keys[m.activeKeyID])
	if err != nil {
		return nil, fmt.Errorf("auth: failed to sign access token: %w", err)
	}

	// Refresh Token
	refreshID := uuid.New().String()
	refreshClaims := jwt.MapClaims{
		"sub": userID,
		"iss": m.config.JWTIssuer,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(m.config.RefreshTokenExpiry).Unix(),
		"jti": refreshID,
		"typ": "refresh",
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken.Header["kid"] = m.activeKeyID
	refreshString, err := refreshToken.SignedString(m.keys[m.activeKeyID])
	if err != nil {
		return nil, fmt.Errorf("auth: failed to sign refresh token: %w", err)
	}

	// Store refresh token family in Redis to prevent replay / allow revocation
	redisKey := fmt.Sprintf("auth:refresh:%s:%s", userID, refreshID)
	if m.redisClient != nil {
		if err := m.redisClient.Set(ctx, redisKey, "valid", m.config.RefreshTokenExpiry).Err(); err != nil {
			return nil, fmt.Errorf("auth: failed to store refresh token: %w", err)
		}
	}

	return &TokenPair{
		AccessToken:  accessString,
		RefreshToken: refreshString,
	}, nil
}

// Verify verifies an access token and returns the parsed claims.
func (m *JWTManager) Verify(tokenString string) (*AuthClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			// Fallback to active key for legacy tokens without kid
			return m.keys[m.activeKeyID], nil
		}

		key, exists := m.keys[kid]
		if !exists {
			return nil, fmt.Errorf("unknown key id: %s", kid)
		}

		return key, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	userID, _ := claims["sub"].(string)
	if typ, ok := claims["typ"].(string); ok && typ == "refresh" {
		return nil, fmt.Errorf("refresh token cannot be used as access token")
	}

	return &AuthClaims{
		UserID: userID,
		Claims: claims,
	}, nil
}

// Refresh issues a new token pair using a valid refresh token.
func (m *JWTManager) Refresh(ctx context.Context, refreshTokenString string) (*TokenPair, error) {
	token, err := jwt.Parse(refreshTokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return m.keys[m.activeKeyID], nil
		}

		key, exists := m.keys[kid]
		if !exists {
			return nil, fmt.Errorf("unknown key id: %s", kid)
		}

		return key, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid refresh token claims")
	}
	typ, _ := claims["typ"].(string)
	if subtle.ConstantTimeCompare([]byte(typ), []byte("refresh")) != 1 {
		return nil, fmt.Errorf("invalid refresh token type")
	}

	userID, _ := claims["sub"].(string)
	jti, _ := claims["jti"].(string)
	if userID == "" || jti == "" {
		return nil, fmt.Errorf("invalid refresh token claims")
	}

	if m.redisClient != nil {
		redisKey := fmt.Sprintf("auth:refresh:%s:%s", userID, jti)
		val, err := m.redisClient.Get(ctx, redisKey).Result()
		if err != nil || subtle.ConstantTimeCompare([]byte(val), []byte("valid")) != 1 {
			return nil, fmt.Errorf("refresh token revoked or expired")
		}

		// Rotate: invalidate old refresh token
		m.redisClient.Del(ctx, redisKey)
	}

	// Issue new token pair
	return m.IssueTokenPair(ctx, userID, nil)
}

// RevokeAll revokes all refresh tokens for a given user.
func (m *JWTManager) RevokeAll(ctx context.Context, userID string) error {
	if m.redisClient == nil {
		return nil
	}
	pattern := fmt.Sprintf("auth:refresh:%s:*", userID)
	iter := m.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		m.redisClient.Del(ctx, iter.Val())
	}
	return iter.Err()
}
