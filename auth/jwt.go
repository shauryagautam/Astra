package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/http"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// JWTManager handles issuing and verifying JWT tokens.
type JWTManager struct {
	config      config.AuthConfig
	redisClient *redis.Client
}

// NewJWTManager creates a new JWTManager.
func NewJWTManager(cfg config.AuthConfig, redisClient *redis.Client) *JWTManager {
	return &JWTManager{
		config:      cfg,
		redisClient: redisClient,
	}
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
	accessString, err := accessToken.SignedString([]byte(m.config.JWTSecret))
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
	refreshString, err := refreshToken.SignedString([]byte(m.config.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("auth: failed to sign refresh token: %w", err)
	}

	// Store refresh token family in Redis to prevent replay / allow revocation
	redisKey := fmt.Sprintf("auth:refresh:%s:%s", userID, refreshID)
	if err := m.redisClient.Set(ctx, redisKey, "valid", m.config.RefreshTokenExpiry).Err(); err != nil {
		return nil, fmt.Errorf("auth: failed to store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessString,
		RefreshToken: refreshString,
	}, nil
}

// Verify verifies an access token and returns the parsed claims.
func (m *JWTManager) Verify(tokenString string) (*http.AuthClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.config.JWTSecret), nil
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

	return &http.AuthClaims{
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
		return []byte(m.config.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["typ"] != "refresh" {
		return nil, fmt.Errorf("invalid refresh token type")
	}

	userID := claims["sub"].(string)
	jti := claims["jti"].(string)

	redisKey := fmt.Sprintf("auth:refresh:%s:%s", userID, jti)
	val, err := m.redisClient.Get(ctx, redisKey).Result()
	if err != nil || val != "valid" {
		return nil, fmt.Errorf("refresh token revoked or expired")
	}

	// Rotate: invalidate old refresh token
	m.redisClient.Del(ctx, redisKey)

	// Issue new token pair
	return m.IssueTokenPair(ctx, userID, nil)
}

// RevokeAll revokes all refresh tokens for a given user.
func (m *JWTManager) RevokeAll(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("auth:refresh:%s:*", userID)
	iter := m.redisClient.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		m.redisClient.Del(ctx, iter.Val())
	}
	return iter.Err()
}
