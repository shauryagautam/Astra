package http

import (
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"sync"

	"github.com/shauryagautam/Astra/pkg/engine"
	identityclaims "github.com/shauryagautam/Astra/pkg/identity/claims"
	"github.com/shauryagautam/Astra/pkg/session"
)

type contextKey string

// HTTPError represents an error that occurred during the processing of an HTTP request.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	return e.Message
}

const astraContextKey contextKey = "astra_context"
const AuthUserKey = "astra_auth_user"

// Context represents the Astra-specific request/response context.
// It is recycled via a sync.Pool to minimize GC pressure.
type Context struct {
	Writer     nethttp.ResponseWriter
	Request    *nethttp.Request
	status     int
	written    bool
	params     map[string]string

	// Explicit Dependencies
	ViewEngine engine.ViewEngine
	Translator engine.Translator
	Sessions   engine.SessionStore
}

var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			params: make(map[string]string),
		}
	},
}

// NewContext initializes or retrieves a Context from the pool.
func NewContext(w nethttp.ResponseWriter, r *nethttp.Request) *Context {
	c := contextPool.Get().(*Context)
	c.Writer = w
	c.Request = r
	c.written = false
	c.status = 0
	c.ViewEngine = nil
	c.Translator = nil
	c.Sessions = nil
	// Clear params from previous use
	for k := range c.params {
		delete(c.params, k)
	}
	return c
}

func (c *Context) release() {
	c.Writer = nil
	c.Request = nil
	contextPool.Put(c)
}

// FromRequest retrieves the Astra context from an http.Request.
func FromRequest(r *nethttp.Request) *Context {
	if c, ok := r.Context().Value(astraContextKey).(*Context); ok {
		return c
	}
	return nil
}

// JSON sends a JSON response with an optional status code (defaults to 200).
func (c *Context) JSON(v any, status ...int) error {
	if c.written {
		return nil
	}

	code := nethttp.StatusOK
	if c.status != 0 {
		code = c.status
	}
	if len(status) > 0 {
		code = status[0]
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	c.written = true
	return json.NewEncoder(c.Writer).Encode(v)
}

// Param retrieves a path parameter.
func (c *Context) Param(name string) string {
	// Support mapping '*' to our internal '_wildcard' for ServeMux compatibility
	searchName := name
	if name == "*" {
		searchName = "_wildcard"
	}

	// Support Go 1.22+ ServeMux path values
	if val := c.Request.PathValue(searchName); val != "" {
		return val
	}
	return c.params[name]
}

// SetParam manually sets a path parameter (used by some middlewares).
func (c *Context) SetParam(name, value string) {
	c.params[name] = value
}

// NoContent sends an empty 204 response.
func (c *Context) NoContent() error {
	if c.written {
		return nil
	}
	c.Writer.WriteHeader(nethttp.StatusNoContent)
	c.written = true
	return nil
}

// Query retrieves a URL query parameter.
func (c *Context) Query(name string) string {
	return c.Request.URL.Query().Get(name)
}

// Ctx returns the underlying request context.
func (c *Context) Ctx() context.Context {
	return c.Request.Context()
}

// Status sets the HTTP response status code.
func (c *Context) Status(code int) *Context {
	c.status = code
	return c
}

// Error sends a specific status and message.
func (c *Context) Error(status int, message string) error {
	if c.written {
		return nil
	}
	c.written = true
	nethttp.Error(c.Writer, message, status)
	return fmt.Errorf("http error: %d %s", status, message)
}

// Redirect performs an HTTP redirect.
func (c *Context) Redirect(url string, code int) error {
	if c.written {
		return nil
	}
	c.written = true
	nethttp.Redirect(c.Writer, c.Request, url, code)
	return nil
}

// Set stores a value in the request context (standard lib interop).
func (c *Context) Set(key string, val any) {
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), key, val))
}

// Get retrieves a value from the request context.
func (c *Context) Get(key any) any {
	return c.Request.Context().Value(key)
}

// GetString retrieves a string value from the request context.
func (c *Context) GetString(key string) string {
	if val, ok := c.Get(key).(string); ok {
		return val
	}
	return ""
}

// Bind decodes the request body into v.
func (c *Context) Bind(v any) error {
	return json.NewDecoder(c.Request.Body).Decode(v)
}

// T translates a key using the registered Translator.
func (c *Context) T(key string, args ...any) string {
	if c.Translator == nil {
		return key
	}
	return c.Translator.T("en", key, args...)
}

func (c *Context) Locale() string {
	if locale, ok := c.Get("astra_locale").(string); ok {
		return locale
	}
	return "en"
}

// Session retrieves the session for the current request.
func (c *Context) Session() *session.Session {
	if sess, ok := c.Get("astra.session").(*session.Session); ok {
		return sess
	}
	return nil
}

// Nonce is a helper to retrieve the CSP nonce.
func (c *Context) Nonce() string {
	return c.GetString("csp_nonce")
}

// SendString sends a plain text response.
func (c *Context) SendString(s string) error {
	if c.written {
		return nil
	}

	code := nethttp.StatusOK
	if c.status != 0 {
		code = c.status
	}

	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true
	_, err := c.Writer.Write([]byte(s))
	return err
}

// ClientIP returns the client's IP address.
func (c *Context) ClientIP() string {
	// Simple implementation, in production use X-Forwarded-For if behind proxy
	return c.Request.RemoteAddr
}

// ─── auth.RequestContext Implementation ───────────────────────────────────────

func (c *Context) GetRequest() *nethttp.Request {
	return c.Request
}

func (c *Context) SetAuthUser(claims *identityclaims.AuthClaims) {
	c.Set(AuthUserKey, claims)
}

func (c *Context) AuthUser() *identityclaims.AuthClaims {
	if claims, ok := c.Get(AuthUserKey).(*identityclaims.AuthClaims); ok {
		return claims
	}
	return nil
}

func (c *Context) SetCookie(cookie *nethttp.Cookie) {
	nethttp.SetCookie(c.Writer, cookie)
}

func (c *Context) RegenerateSession() error {
	sess := c.Session()
	if sess != nil {
		return sess.Regenerate(c.Writer)
	}
	return nil
}

// IsAuthenticated returns true if a user is logged in.
func (c *Context) IsAuthenticated() bool {
	return c.AuthUser() != nil
}
