package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/astraframework/astra/audit"
	"github.com/astraframework/astra/events"
)

// RequestContext defines the minimal interface required by auth guards.
type RequestContext interface {
	GetRequest() *nethttp.Request
	SetAuthUser(claims *AuthClaims)
	SetCookie(cookie *nethttp.Cookie)
	RegenerateSession() error
}

// Guard is the interface that auth guards must implement.
type Guard interface {
	Attempt(c RequestContext) error
}

// JWTGuard implements Guard for JWT tokens via Authorization Header.
type JWTGuard struct {
	Manager *JWTManager
}

// Attempt validates the JWT from the Authorization header and sets the user context.
func (g *JWTGuard) Attempt(c RequestContext) error {
	req := c.GetRequest()
	authHeader := req.Header.Get("Authorization")
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
		events.DefaultEmitter.Emit(req.Context(), audit.AuditEvent{
			Action:    "login",
			Success:   false,
			Error:     err.Error(),
			IPAddress: req.RemoteAddr,
			UserAgent: req.UserAgent(),
		})
		return err
	}

	events.DefaultEmitter.Emit(req.Context(), audit.AuditEvent{
		ActorID:   claims.UserID,
		Action:    "login",
		Success:   true,
		IPAddress: req.RemoteAddr,
		UserAgent: req.UserAgent(),
	})

	c.SetAuthUser(claims)

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
func (g *CookieGuard) Attempt(c RequestContext) error {
	req := c.GetRequest()
	cookie, err := req.Cookie(g.CookieName)
	if err != nil {
		return err // Missing cookie
	}

	token := cookie.Value
	data, err := g.Session.Get(req.Context(), token)
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

	claims := &AuthClaims{
		UserID: userID,
	}
	c.SetAuthUser(claims)

	return nil
}

// Login creates a new session and issues a cookie.
// It rotates both the auth token and the underlying web session ID.
func (g *CookieGuard) Login(c RequestContext, userID string) error {
	req := c.GetRequest()

	// 1. Revoke old auth session if it exists (prevents orphan sessions)
	if oldCookie, err := req.Cookie(g.CookieName); err == nil {
		_ = g.Session.Destroy(req.Context(), oldCookie.Value)
	}

	// 2. Rotate web session ID (prevents session fixation)
	_ = c.RegenerateSession()

	// 3. Issue new auth token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("auth: failed to generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	ttl := 24 * time.Hour
	err := g.Session.Set(req.Context(), token, map[string]any{"userID": userID}, ttl)
	if err != nil {
		return err
	}

	c.SetCookie(&nethttp.Cookie{
		Name:     g.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		Secure:   req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: nethttp.SameSiteLaxMode,
	})

	return nil
}

// Logout revokes the session and clears the cookie.
func (g *CookieGuard) Logout(c RequestContext) error {
	req := c.GetRequest()
	cookie, err := req.Cookie(g.CookieName)
	if err == nil {
		_ = g.Session.Destroy(req.Context(), cookie.Value)
	}

	c.SetCookie(&nethttp.Cookie{
		Name:   g.CookieName,
		Value:  "",
		MaxAge: -1,
	})

	events.DefaultEmitter.Emit(req.Context(), audit.AuditEvent{
		Action:    "logout",
		Success:   true,
		IPAddress: req.RemoteAddr,
		UserAgent: req.UserAgent(),
	})

	return nil
}
