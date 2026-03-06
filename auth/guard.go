package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/astraframework/astra/http"
)

// Guard is the interface that auth guards must implement.
type Guard interface {
	Attempt(c *http.Context) error
}

// JWTGuard implements Guard for JWT tokens via Authorization Header.
type JWTGuard struct {
	Manager *JWTManager
}

// Attempt validates the JWT from the Authorization header and sets the user context.
func (g *JWTGuard) Attempt(c *http.Context) error {
	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		return errors.New("missing authorization header")
	}

	const prefix = "Bearer "
	if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
		return errors.New("invalid authorization header format")
	}

	token := authHeader[len(prefix):]
	claims, err := g.Manager.Verify(token)
	if err != nil {
		return err
	}

	c.SetAuthUser(claims)
	
	// Also set in the standard request context for GraphQL and others to use via auth.GetAuthUser
	ctx := context.WithValue(c.Request.Context(), UserKey, claims)
	c.Request = c.Request.WithContext(ctx)
	
	return nil
}

// CookieGuard implements Guard for Session cookies using Redis mapping.
type CookieGuard struct {
	Session    SessionDriver
	CookieName string
}

// NewCookieGuard creates a new CookieGuard.
func NewCookieGuard(session SessionDriver) *CookieGuard {
	return &CookieGuard{
		Session:    session,
		CookieName: "astra_session",
	}
}

// Attempt validates the session cookie.
func (g *CookieGuard) Attempt(c *http.Context) error {
	cookie, err := c.Request.Cookie(g.CookieName)
	if err != nil {
		return err // Missing cookie
	}

	token := cookie.Value
	data, err := g.Session.Get(c.Request.Context(), token)
	if err != nil {
		return errors.New("invalid or expired session")
	}

	userIDMatches, ok := data["userID"]
	if !ok {
		return errors.New("session payload invalid")
	}

	var userID string
	if strID, ok := userIDMatches.(string); ok {
		userID = strID
	} else {
		userID = fmt.Sprintf("%v", userIDMatches)
	}

	claims := &http.AuthClaims{
		UserID: userID,
	}
	c.SetAuthUser(claims)
	
	// Also set in the standard request context
	ctx := context.WithValue(c.Request.Context(), UserKey, claims)
	c.Request = c.Request.WithContext(ctx)
	
	return nil
}

// Login creates a new session and issues a cookie.
func (g *CookieGuard) Login(c *http.Context, userID string) error {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	ttl := 24 * time.Hour
	err := g.Session.Set(c.Request.Context(), token, map[string]any{"userID": userID}, ttl)
	if err != nil {
		return err
	}

	nethttp.SetCookie(c.Writer, &nethttp.Cookie{
		Name:     g.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: nethttp.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
	})

	return nil
}

// Logout revokes the session and clears the cookie.
func (g *CookieGuard) Logout(c *http.Context) error {
	cookie, err := c.Request.Cookie(g.CookieName)
	if err == nil {
		_ = g.Session.Destroy(c.Request.Context(), cookie.Value)
	}

	nethttp.SetCookie(c.Writer, &nethttp.Cookie{
		Name:     g.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: nethttp.SameSiteLaxMode,
		MaxAge:   -1,
	})

	return nil
}
