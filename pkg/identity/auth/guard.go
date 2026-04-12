package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	nethttp "net/http"
	"strconv"
	"time"

	"github.com/shauryagautam/Astra/pkg/observability/audit"
	"github.com/shauryagautam/Astra/pkg/engine/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	identityclaims "github.com/shauryagautam/Astra/pkg/identity/claims"
	"sync"
)

var (
	mu     sync.RWMutex
	guards = make(map[string]Guard)
)

// Register registers a guard by name.
func Register(name string, g Guard) {
	mu.Lock()
	defer mu.Unlock()
	guards[name] = g
}

// Resolve retrieves a registered guard by name.
func Resolve(name string) Guard {
	mu.RLock()
	defer mu.RUnlock()
	return guards[name]
}

// GetGuard is an alias for Resolve.
func GetGuard(name string) Guard {
	return Resolve(name)
}

// GetAuthUser retrieves the AuthClaims from the given context.
func GetAuthUser(ctx context.Context) *identityclaims.AuthClaims {
	if claims, ok := ctx.Value("astra_auth_user").(*identityclaims.AuthClaims); ok {
		return claims
	}
	return nil
}

// RequestContext defines the minimal interface required by auth guards.
type RequestContext interface {
	GetRequest() *nethttp.Request
	SetAuthUser(claims *identityclaims.AuthClaims)
	SetCookie(cookie *nethttp.Cookie)
	RegenerateSession() error
}

// Guard is the interface that auth guards must implement.
type Guard interface {
	Name() string
	Attempt(c RequestContext) error
	Login(c RequestContext, user any) (any, error)
	Logout(c RequestContext) error
}

// JWTGuard implements Guard for JWT tokens via Authorization Header.
type JWTGuard struct {
	name    string
	Manager *JWTManager
}

func NewJWTGuard(name string, mgr *JWTManager) *JWTGuard {
	return &JWTGuard{name: name, Manager: mgr}
}

func (g *JWTGuard) Name() string { return g.name }

// Attempt validates the JWT from the Authorization header and sets the user context.
func (g *JWTGuard) Attempt(c RequestContext) error {
	req := c.GetRequest()
	
	tracer := otel.Tracer("astra.auth")
	ctx, span := tracer.Start(req.Context(), "auth.guard.jwt", trace.WithAttributes(
		attribute.String("security.event", "authentication_attempt"),
		attribute.String("auth.method", "jwt"),
		attribute.String("network.client.ip", req.RemoteAddr),
	))
	defer span.End()

	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		span.SetAttributes(attribute.Bool("auth.success", false), attribute.String("auth.reason", "missing_header"))
		return errors.New("missing authorization header")
	}

	const prefix = "Bearer "
	if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
		span.SetAttributes(attribute.Bool("auth.success", false), attribute.String("auth.reason", "invalid_format"))
		return errors.New("invalid authorization header format")
	}

	token := authHeader[len(prefix):]
	claims, err := g.Manager.Verify(token)
	if err != nil {
		span.SetAttributes(attribute.Bool("auth.success", false), attribute.String("auth.reason", err.Error()))
		event.DefaultEmitter.Emit(ctx, audit.AuditEvent{
			Action:    "login",
			Success:   false,
			Error:     err.Error(),
			IPAddress: req.RemoteAddr,
			UserAgent: req.UserAgent(),
		})
		return err
	}

	span.SetAttributes(
		attribute.Bool("auth.success", true),
		attribute.String("user.id", claims.UserID),
	)

	event.DefaultEmitter.Emit(ctx, audit.AuditEvent{
		ActorID:   claims.UserID,
		Action:    "login",
		Success:   true,
		IPAddress: req.RemoteAddr,
		UserAgent: req.UserAgent(),
	})

	c.SetAuthUser(claims)

	return nil
}

func (g *JWTGuard) Login(c RequestContext, user any) (any, error) {
	// user should be an ID string or have an ID field
	var userID string
	switch v := user.(type) {
	case string:
		userID = v
	case interface{ GetID() string }:
		userID = v.GetID()
	default:
		return nil, errors.New("jwt: user must be a string ID or implement GetID()")
	}

	pair, err := g.Manager.IssueTokenPair(c.GetRequest().Context(), userID, nil)
	if err != nil {
		return nil, err
	}
	return pair.AccessToken, nil
}

func (g *JWTGuard) Logout(c RequestContext) error {
	// JWT is stateless, but we could blacklist here if needed
	return nil
}


// CookieGuard implements Guard for Session cookies using Redis mapping.
type CookieGuard struct {
	name       string
	Session    SessionDriver
	CookieName string
}

// NewCookieGuard creates a new CookieGuard.
func NewCookieGuard(name string, session SessionDriver) *CookieGuard {
	return &CookieGuard{
		name:       name,
		Session:    session,
		CookieName: "astra_session",
	}
}

func (g *CookieGuard) Name() string { return g.name }

// Attempt validates the session cookie.
func (g *CookieGuard) Attempt(c RequestContext) error {
	req := c.GetRequest()

	tracer := otel.Tracer("astra.auth")
	ctx, span := tracer.Start(req.Context(), "auth.guard.cookie", trace.WithAttributes(
		attribute.String("security.event", "authentication_attempt"),
		attribute.String("auth.method", "cookie"),
		attribute.String("network.client.ip", req.RemoteAddr),
	))
	defer span.End()

	cookie, err := req.Cookie(g.CookieName)
	if err != nil {
		span.SetAttributes(attribute.Bool("auth.success", false), attribute.String("auth.reason", "missing_cookie"))
		return err // Missing cookie
	}

	token := cookie.Value
	data, err := g.Session.Get(ctx, token)
	if err != nil {
		span.SetAttributes(attribute.Bool("auth.success", false), attribute.String("auth.reason", "invalid_session"))
		return errors.New("invalid or expired session")
	}

	userIDMatches, ok := data["userID"]
	if !ok {
		span.SetAttributes(attribute.Bool("auth.success", false), attribute.String("auth.reason", "payload_invalid"))
		return errors.New("session payload invalid")
	}

	var userID string
	switch id := userIDMatches.(type) {
	case string:
		userID = id
	case float64:
		userID = strconv.FormatFloat(id, 'f', 0, 64)
	case int:
		userID = strconv.Itoa(id)
	case int64:
		userID = strconv.FormatInt(id, 10)
	case int32:
		userID = strconv.FormatInt(int64(id), 10)
	case uint:
		userID = strconv.FormatUint(uint64(id), 10)
	case uint64:
		userID = strconv.FormatUint(id, 10)
	default:
		span.SetAttributes(attribute.Bool("auth.success", false), attribute.String("auth.reason", "unsafe_payload"))
		return errors.New("unsafe session payload: userID type not explicitly supported")
	}

	span.SetAttributes(
		attribute.Bool("auth.success", true),
		attribute.String("user.id", userID),
	)

	claims := &identityclaims.AuthClaims{
		UserID: userID,
	}
	c.SetAuthUser(claims)

	return nil
}


// Login creates a new session and issues a cookie.
// It rotates both the auth token and the underlying web session ID.
func (g *CookieGuard) Login(c RequestContext, user any) (any, error) {
	var userID string
	switch v := user.(type) {
	case string:
		userID = v
	case interface{ GetID() string }:
		userID = v.GetID()
	default:
		return nil, errors.New("cookie: user must be a string ID or implement GetID()")
	}
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
		return nil, fmt.Errorf("auth: failed to generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	ttl := 24 * time.Hour
	err := g.Session.Set(req.Context(), token, map[string]any{"userID": userID}, ttl)
	if err != nil {
		return nil, err
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

	return nil, nil
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

	event.DefaultEmitter.Emit(req.Context(), audit.AuditEvent{
		Action:    "logout",
		Success:   true,
		IPAddress: req.RemoteAddr,
		UserAgent: req.UserAgent(),
	})

	return nil
}
