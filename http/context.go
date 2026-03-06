package http

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/storage"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey string

const (
	authUserKey   contextKey = "astra_auth_user"
	flashPrefix   = "astra_flash_"
)

// Context is the central request context object passed to all Astra handlers.
// It wraps the standard http.Request and http.ResponseWriter with convenience
// methods for binding, validation, responses, auth, and pagination.
type Context struct {
	Request *http.Request
	Writer  http.ResponseWriter
	App     *core.App

	// internal
	values  map[string]any
	written bool
}

// contextPool reuses Context objects to minimize allocations on hot paths.
var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			values: make(map[string]any, 8),
		}
	},
}

// NewContext creates a new Context for the given request/response pair.
func NewContext(w http.ResponseWriter, r *http.Request, app *core.App) *Context {
	c := contextPool.Get().(*Context)
	c.Request = r
	c.Writer = w
	c.App = app
	c.written = false
	// Reset values map
	for k := range c.values {
		delete(c.values, k)
	}
	return c
}

// release returns the Context to the pool. Called automatically after request handling.
func (c *Context) release() {
	c.Request = nil
	c.Writer = nil
	c.App = nil
	contextPool.Put(c)
}

// ─── Binding & Validation ─────────────────────────────────────────────

// Bind decodes the JSON request body into the given struct.
func (c *Context) Bind(v any) error {
	if c.Request.Body == nil {
		return NewHTTPError(http.StatusBadRequest, "EMPTY_BODY", "request body is empty")
	}
	defer c.Request.Body.Close()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return NewHTTPError(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body exceeds size limit")
		}
		return NewHTTPError(http.StatusBadRequest, "READ_ERROR", "failed to read request body")
	}

	if len(body) == 0 {
		return NewHTTPError(http.StatusBadRequest, "EMPTY_BODY", "request body is empty")
	}

	if err := sonic.Unmarshal(body, v); err != nil {
		return NewHTTPError(http.StatusBadRequest, "INVALID_JSON", "malformed JSON in request body")
	}

	return nil
}

// BindAndValidate decodes the JSON body and validates using go-playground/validator tags.
// Validation errors are returned as a structured ValidationError.
func (c *Context) BindAndValidate(v any) error {
	if err := c.Bind(v); err != nil {
		return err
	}

	if c.App != nil {
		if valSvc := c.App.Get("validator"); valSvc != nil {
			if validator, ok := valSvc.(interface{ ValidateStruct(any) error }); ok {
				return validator.ValidateStruct(v)
			}
		}
	}
	return Validate(v)
}

// ─── Parameters ───────────────────────────────────────────────────────

// Param returns a URL path parameter value (from chi's URL params).
func (c *Context) Param(key string) string {
	return chi.URLParam(c.Request, key)
}

// Query returns a query string parameter value.
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// QueryDefault returns a query string parameter value or a default if empty.
func (c *Context) QueryDefault(key string, def string) string {
	val := c.Query(key)
	if val == "" {
		return def
	}
	return val
}

// QueryInt returns a query string parameter as an integer, or the default if
// not set or not a valid integer.
func (c *Context) QueryInt(key string, def int) int {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return val
}

// QueryBool returns a query string parameter as a boolean, or the default.
func (c *Context) QueryBool(key string, def bool) bool {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	switch strings.ToLower(raw) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}

// ─── Response ─────────────────────────────────────────────────────────

// JSON sends a JSON response with the given data and optional status code.
// Default status is 200 OK.
func (c *Context) JSON(data any, status ...int) error {
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}

	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true
	return sonic.ConfigDefault.NewEncoder(c.Writer).Encode(data)
}

// Error sends a structured JSON error response.
func (c *Context) Error(code int, message string) error {
	return c.JSON(map[string]any{
		"error": map[string]any{
			"code":    http.StatusText(code),
			"message": message,
		},
	}, code)
}

// ValidationError sends a structured 422 validation error response.
func (c *Context) ValidationError(err error) error {
	if ve, ok := err.(*ValidationErrors); ok {
		return c.JSON(map[string]any{
			"error": map[string]any{
				"code":    "VALIDATION_ERROR",
				"message": "The given data was invalid.",
				"fields":  ve.Fields,
			},
		}, http.StatusUnprocessableEntity)
	}
	return c.Error(http.StatusUnprocessableEntity, err.Error())
}

// Created sends a 201 Created response with the given data.
func (c *Context) Created(data any) error {
	return c.JSON(data, http.StatusCreated)
}

// NoContent sends a 204 No Content response.
func (c *Context) NoContent() error {
	c.Writer.WriteHeader(http.StatusNoContent)
	c.written = true
	return nil
}

// Redirect sends a redirect response. Default status is 302 Found.
func (c *Context) Redirect(url string, status ...int) error {
	code := http.StatusFound
	if len(status) > 0 {
		code = status[0]
	}
	http.Redirect(c.Writer, c.Request, url, code)
	c.written = true
	return nil
}

// SendString sends a plain text response.
func (c *Context) SendString(s string, status ...int) error {
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true
	_, err := c.Writer.Write([]byte(s))
	return err
}

// HTML sends an HTML response.
func (c *Context) HTML(html string, status ...int) error {
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true
	_, err := c.Writer.Write([]byte(html))
	return err
}

// File sends a file response.
func (c *Context) File(filepath string) error {
	http.ServeFile(c.Writer, c.Request, filepath)
	c.written = true
	return nil
}

// ─── Flash Messages ───────────────────────────────────────────────────

const flashCookiePrefix = "astra_flash_"

// Flash sets a flash message that persists only for the next request.
// It uses an HTTP-only cookie.
func (c *Context) Flash(name string, value string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     flashCookiePrefix + name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600, // 1 hour is plenty for a flash message
	})
}

// GetFlash retrieves a flash message and clears it.
func (c *Context) GetFlash(name string) string {
	cookie, err := c.Request.Cookie(flashCookiePrefix + name)
	if err != nil {
		return ""
	}

	// Clear the cookie immediately
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     flashCookiePrefix + name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	return cookie.Value
}

// FormFile returns the uploaded file for the given form field.
func (c *Context) FormFile(field string) (*UploadedFile, error) {
	file, header, err := c.Request.FormFile(field)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return &UploadedFile{
		Name:    header.Filename,
		Size:    header.Size,
		Content: content,
		Header:  header,
	}, nil
}

// UploadedFile represents an uploaded file.
type UploadedFile struct {
	Name    string
	Size    int64
	Content []byte
	Header  *multipart.FileHeader
}

// Store saves the uploaded file to the given storage driver.
func (f *UploadedFile) Store(ctx context.Context, s storage.Storage, path string) error {
	return s.Put(ctx, path, f.Content)
}

// Download forces the browser to download the file at the given filepath.
func (c *Context) Download(filepath string, filename ...string) error {
	name := ""
	if len(filename) > 0 {
		name = filename[0]
	} else {
		// Extract filename from path if not provided
		parts := strings.Split(filepath, "/")
		name = parts[len(parts)-1]
	}
	c.Writer.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(c.Writer, c.Request, filepath)
	c.written = true
	return nil
}

// ─── Auth ─────────────────────────────────────────────────────────────

// AuthUser returns the authenticated user's claims from the context.
// Returns nil if no authenticated user is set.
func (c *Context) AuthUser() *AuthClaims {
	claims, _ := c.Request.Context().Value(authUserKey).(*AuthClaims)
	return claims
}

// IsAuthenticated returns true if the request has an authenticated user.
func (c *Context) IsAuthenticated() bool {
	return c.AuthUser() != nil
}

// SetAuthUser stores the authenticated user's claims in the request context.
func (c *Context) SetAuthUser(claims *AuthClaims) {
	ctx := context.WithValue(c.Request.Context(), authUserKey, claims)
	c.Request = c.Request.WithContext(ctx)
}

// AuthClaims holds the authenticated user's information extracted from JWT.
type AuthClaims struct {
	UserID string
	Email  string
	Claims map[string]any
}

// Can checks if the authenticated user is allowed to perform the action.
func (c *Context) Can(action string, subject any) bool {
	user := c.AuthUser()
	if user == nil {
		return false
	}
	if c.App != nil && c.App.Gate != nil {
		return c.App.Gate.Allows(user, action, subject)
	}
	// Fallback, though should not be reachable in correct setups
	return false
}

// Authorize checks if the authenticated user is allowed to perform the action and returns an error if not.
func (c *Context) Authorize(action string, subject any) error {
	if c.Can(action, subject) {
		return nil
	}
	return c.Error(http.StatusForbidden, "not authorized to "+action)
}

// ─── Pagination ───────────────────────────────────────────────────────

// Page reads the ?page= query parameter, defaulting to 1.
func (c *Context) Page() int {
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	return page
}

// PerPage reads the ?per_page= query parameter, capped at the given max.
func (c *Context) PerPage(max int) int {
	perPage := c.QueryInt("per_page", 15)
	if perPage < 1 {
		perPage = 15
	}
	if perPage > max {
		perPage = max
	}
	return perPage
}

// ─── Context Propagation ──────────────────────────────────────────────

// Ctx returns the underlying context.Context from the request.
func (c *Context) Ctx() context.Context {
	return c.Request.Context()
}

// Set stores a value in the context's local value map.
func (c *Context) Set(key string, val any) {
	c.values[key] = val
}

// Get retrieves a value from the context's local value map.
func (c *Context) Get(key string) any {
	return c.values[key]
}

// GetString retrieves a string value from the context's local value map.
func (c *Context) GetString(key string) string {
	val, _ := c.values[key].(string)
	return val
}

// ─── Request Helpers ──────────────────────────────────────────────────

// Header returns the value of the given request header.
func (c *Context) Header(key string) string {
	return c.Request.Header.Get(key)
}

// SetHeader sets a response header.
func (c *Context) SetHeader(key, value string) {
	c.Writer.Header().Set(key, value)
}

// ClientIP returns the client's IP address, respecting proxy headers.
func (c *Context) ClientIP() string {
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := c.Request.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	host, _, _ := strings.Cut(c.Request.RemoteAddr, ":")
	return host
}

// IsWritten returns true if a response has already been sent.
func (c *Context) IsWritten() bool {
	return c.written
}

// ─── Authorization ────────────────────────────────────────────────────

// Forbidden sends a 403 Forbidden JSON error response and returns it as an error.
func (c *Context) Forbidden(message string) error {
	if message == "" {
		message = "You are not authorized to perform this action."
	}
	return NewHTTPError(http.StatusForbidden, "FORBIDDEN", message)
}

// ─── Cursor Pagination ────────────────────────────────────────────────

// Cursor reads the ?cursor= query parameter for cursor-based pagination.
func (c *Context) Cursor() string {
	return c.Query("cursor")
}

// CursorLimit reads the ?limit= query parameter, capped at the given max.
// Defaults to 15 if not set or invalid.
func (c *Context) CursorLimit(max int) int {
	limit := c.QueryInt("limit", 15)
	if limit < 1 {
		limit = 15
	}
	if limit > max {
		limit = max
	}
	return limit
}
