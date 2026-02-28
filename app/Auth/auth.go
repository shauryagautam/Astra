// Package auth provides authentication guards for Adonis Go.
// Mirrors AdonisJS's @adonisjs/auth with JWT and OAT guards.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/shaurya/adonis/contracts"
)

// Common errors.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNotAuthenticated   = errors.New("not authenticated")
	ErrTokenExpired       = errors.New("token has expired")
	ErrInvalidToken       = errors.New("invalid token")
)

// Token represents an authentication token.
type Token struct {
	TokenType string `json:"type"`
	TokenName string `json:"name,omitempty"`
	TokenVal  string `json:"token"`
	ExpiresIn int64  `json:"expires_at,omitempty"`
}

func (t *Token) Type() string     { return t.TokenType }
func (t *Token) Name() string     { return t.TokenName }
func (t *Token) Token() string    { return t.TokenVal }
func (t *Token) ExpiresAt() int64 { return t.ExpiresIn }

var _ contracts.TokenContract = (*Token)(nil)

// ══════════════════════════════════════════════════════════════════════
// JWT Guard
// Mirrors AdonisJS's JWT web guard.
// ══════════════════════════════════════════════════════════════════════

// JWTConfig holds JWT guard configuration.
type JWTConfig struct {
	Secret string
	Expiry time.Duration
	Issuer string
}

type JWTGuard struct {
	config    JWTConfig
	provider  contracts.UserProviderContract
	blacklist contracts.BlacklistContract
	user      contracts.Authenticatable
}

// NewJWTGuard creates a new JWT guard.
func NewJWTGuard(config JWTConfig, provider contracts.UserProviderContract, blacklist contracts.BlacklistContract) *JWTGuard {
	return &JWTGuard{
		config:    config,
		provider:  provider,
		blacklist: blacklist,
	}
}

// Attempt authenticates via credentials and returns a JWT token.
func (g *JWTGuard) Attempt(ctx contracts.HttpContextContract, credentials map[string]any) (contracts.TokenContract, error) {
	user, err := g.provider.FindByCredentials(ctx.Context(), credentials)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	return g.Login(ctx, user)
}

// Login generates a JWT token for an authenticated user.
func (g *JWTGuard) Login(ctx contracts.HttpContextContract, user contracts.Authenticatable) (contracts.TokenContract, error) {
	now := time.Now()
	expiresAt := now.Add(g.config.Expiry)

	claims := jwt.MapClaims{
		"sub": fmt.Sprintf("%v", user.GetAuthIdentifier()),
		"iss": g.config.Issuer,
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(g.config.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign JWT: %w", err)
	}

	g.user = user

	return &Token{
		TokenType: "bearer",
		TokenVal:  tokenString,
		ExpiresIn: expiresAt.Unix(),
	}, nil
}

// Authenticate verifies the JWT token from the request and loads the user.
func (g *JWTGuard) Authenticate(ctx contracts.HttpContextContract) (contracts.Authenticatable, error) {
	tokenString := extractBearerToken(ctx)
	if tokenString == "" {
		return nil, ErrNotAuthenticated
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(g.config.Secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	// Check if token is blacklisted
	if g.blacklist != nil {
		blacklisted, err := g.blacklist.Has(ctx.Context(), tokenString)
		if err != nil || blacklisted {
			return nil, ErrInvalidToken
		}
	}

	user, err := g.provider.FindById(ctx.Context(), sub)
	if err != nil {
		return nil, ErrNotAuthenticated
	}

	g.user = user
	ctx.WithValue("auth_user", user)
	return user, nil
}

// Check returns true if the request is authenticated.
func (g *JWTGuard) Check(ctx contracts.HttpContextContract) bool {
	_, err := g.Authenticate(ctx)
	return err == nil
}

// Logout invalidates the JWT by adding it to the blacklist.
func (g *JWTGuard) Logout(ctx contracts.HttpContextContract) error {
	tokenString := extractBearerToken(ctx)
	if tokenString == "" {
		return nil
	}

	// Calculate remaining TTL
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err == nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if exp, ok := claims["exp"].(float64); ok {
				remaining := time.Until(time.Unix(int64(exp), 0))
				if remaining > 0 && g.blacklist != nil {
					_ = g.blacklist.Add(ctx.Context(), tokenString, remaining)
				}
			}
		}
	}

	g.user = nil
	return nil
}

// User returns the authenticated user.
func (g *JWTGuard) User() contracts.Authenticatable {
	return g.user
}

var _ contracts.GuardContract = (*JWTGuard)(nil)

// ══════════════════════════════════════════════════════════════════════
// OAT (Opaque Access Token) Guard
// Mirrors AdonisJS's OAT guard (API tokens stored in database/Redis).
// ══════════════════════════════════════════════════════════════════════

// TokenStore is the interface for storing opaque tokens.
// Can be backed by database, Redis, or in-memory.
type TokenStore interface {
	// Store saves a token associated with a user.
	Store(userID string, token string, name string, expiresAt time.Time) error

	// Find looks up a token and returns the associated user ID.
	Find(token string) (userID string, err error)

	// Revoke deletes a specific token.
	Revoke(token string) error

	// RevokeAll deletes all tokens for a user.
	RevokeAll(userID string) error
}

// OATGuard implements Opaque Access Token authentication.
type OATGuard struct {
	expiry   time.Duration
	store    TokenStore
	provider contracts.UserProviderContract
	user     contracts.Authenticatable
}

// NewOATGuard creates a new OAT guard.
func NewOATGuard(expiry time.Duration, store TokenStore, provider contracts.UserProviderContract) *OATGuard {
	return &OATGuard{
		expiry:   expiry,
		store:    store,
		provider: provider,
	}
}

// Attempt authenticates via credentials and returns an opaque token.
func (g *OATGuard) Attempt(ctx contracts.HttpContextContract, credentials map[string]any) (contracts.TokenContract, error) {
	user, err := g.provider.FindByCredentials(ctx.Context(), credentials)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	return g.Login(ctx, user)
}

// Login generates an opaque token for the user.
func (g *OATGuard) Login(ctx contracts.HttpContextContract, user contracts.Authenticatable) (contracts.TokenContract, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	tokenString := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(g.expiry)
	userID := fmt.Sprintf("%v", user.GetAuthIdentifier())

	if err := g.store.Store(userID, tokenString, "api", expiresAt); err != nil {
		return nil, fmt.Errorf("failed to store token: %w", err)
	}

	g.user = user

	return &Token{
		TokenType: "bearer",
		TokenName: "api",
		TokenVal:  tokenString,
		ExpiresIn: expiresAt.Unix(),
	}, nil
}

// Authenticate verifies the opaque token from the request.
func (g *OATGuard) Authenticate(ctx contracts.HttpContextContract) (contracts.Authenticatable, error) {
	tokenString := extractBearerToken(ctx)
	if tokenString == "" {
		return nil, ErrNotAuthenticated
	}

	userID, err := g.store.Find(tokenString)
	if err != nil {
		return nil, ErrInvalidToken
	}

	user, err := g.provider.FindById(ctx.Context(), userID)
	if err != nil {
		return nil, ErrNotAuthenticated
	}

	g.user = user
	ctx.WithValue("auth_user", user)
	return user, nil
}

// Check returns true if the request is authenticated.
func (g *OATGuard) Check(ctx contracts.HttpContextContract) bool {
	_, err := g.Authenticate(ctx)
	return err == nil
}

// Logout revokes the current token.
func (g *OATGuard) Logout(ctx contracts.HttpContextContract) error {
	tokenString := extractBearerToken(ctx)
	if tokenString == "" {
		return nil
	}
	g.user = nil
	return g.store.Revoke(tokenString)
}

// User returns the authenticated user.
func (g *OATGuard) User() contracts.Authenticatable {
	return g.user
}

var _ contracts.GuardContract = (*OATGuard)(nil)

// ══════════════════════════════════════════════════════════════════════
// Auth Manager
// Manages multiple auth guards. Mirrors AdonisJS's Auth module.
// ══════════════════════════════════════════════════════════════════════

// AuthManager holds all configured guards.
type AuthManager struct {
	defaultGuard string
	guards       map[string]contracts.GuardContract
}

// NewAuthManager creates a new Auth manager.
func NewAuthManager(defaultGuard string) *AuthManager {
	return &AuthManager{
		defaultGuard: defaultGuard,
		guards:       make(map[string]contracts.GuardContract),
	}
}

// RegisterGuard adds a guard to the manager.
func (m *AuthManager) RegisterGuard(name string, guard contracts.GuardContract) {
	m.guards[name] = guard
}

// Use selects a guard by name.
func (m *AuthManager) Use(guard string) contracts.GuardContract {
	if g, ok := m.guards[guard]; ok {
		return g
	}
	panic(fmt.Sprintf("Auth guard '%s' not found", guard))
}

// DefaultGuard returns the default guard.
func (m *AuthManager) DefaultGuard() contracts.GuardContract {
	return m.Use(m.defaultGuard)
}

var _ contracts.AuthManagerContract = (*AuthManager)(nil)

// ══════════════════════════════════════════════════════════════════════
// In-Memory Token Store (for development/testing)
// ══════════════════════════════════════════════════════════════════════

// MemoryTokenStore stores tokens in memory. For development/testing only.
type MemoryTokenStore struct {
	tokens map[string]memoryToken
}

type memoryToken struct {
	UserID    string
	Name      string
	ExpiresAt time.Time
}

// NewMemoryTokenStore creates a new in-memory token store.
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{
		tokens: make(map[string]memoryToken),
	}
}

func (s *MemoryTokenStore) Store(userID string, token string, name string, expiresAt time.Time) error {
	s.tokens[token] = memoryToken{UserID: userID, Name: name, ExpiresAt: expiresAt}
	return nil
}

func (s *MemoryTokenStore) Find(token string) (string, error) {
	t, ok := s.tokens[token]
	if !ok {
		return "", ErrInvalidToken
	}
	if time.Now().After(t.ExpiresAt) {
		delete(s.tokens, token)
		return "", ErrTokenExpired
	}
	return t.UserID, nil
}

func (s *MemoryTokenStore) Revoke(token string) error {
	delete(s.tokens, token)
	return nil
}

func (s *MemoryTokenStore) RevokeAll(userID string) error {
	for k, v := range s.tokens {
		if v.UserID == userID {
			delete(s.tokens, k)
		}
	}
	return nil
}

var _ TokenStore = (*MemoryTokenStore)(nil)

// ══════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════

// extractBearerToken extracts the Bearer token from the Authorization header.
func extractBearerToken(ctx contracts.HttpContextContract) string {
	header := ctx.Request().Header("Authorization")
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return ""
}
