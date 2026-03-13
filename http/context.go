package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/astraframework/astra/auth"
	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey string

const (
	authUserKey contextKey = "astra_auth_user"
)

// Context is the central request context object passed to all Astra handlers.
// It wraps the standard http.Request and http.ResponseWriter with convenience
// methods for binding, validation, responses, auth, and pagination.
type Context struct {
	Request *http.Request
	Writer  http.ResponseWriter
	App     *core.App

	// internal
	values     map[string]any
	written    bool
	resp       *responseWriter
	queryCache map[string][]string
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// contextPool reuses Context objects to minimize allocations on hot paths.
var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			values: make(map[string]any, 8),
		}
	},
}

// responseWriterPool reuses responseWriter objects.
var responseWriterPool = sync.Pool{
	New: func() any {
		return &responseWriter{}
	},
}

// NewContext creates a new Context for the given request/response pair.
func NewContext(w http.ResponseWriter, r *http.Request, app *core.App) *Context {
	c := contextPool.Get().(*Context)
	rw := responseWriterPool.Get().(*responseWriter)
	rw.ResponseWriter = w
	rw.status = 0

	c.Request = r
	c.Writer = rw
	c.resp = rw
	c.App = app
	c.written = false
	c.queryCache = nil
	// Reset values map efficiently
	if len(c.values) > 0 {
		for k := range c.values {
			delete(c.values, k)
		}
	}
	return c
}

// GetRequest returns the underlying http.Request.
func (c *Context) GetRequest() *http.Request {
	return c.Request
}

// release returns the Context to the pool. Called automatically after request handling.
func (c *Context) release() {
	rw := c.resp
	c.Request = nil
	c.Writer = nil
	c.resp = nil
	c.App = nil
	c.queryCache = nil

	contextPool.Put(c)

	if rw != nil {
		rw.ResponseWriter = nil
		responseWriterPool.Put(rw)
	}
}

// ─── Binding & Validation ─────────────────────────────────────────────

// Bind decodes the JSON request body into the given struct.
// Uses Sonic JSON for ultra-fast deserialization.
func (c *Context) Bind(v any) error {
	if c.Request.Body == nil {
		return NewHTTPError(http.StatusBadRequest, "EMPTY_BODY", "request body is empty")
	}
	defer func() {
		if err := c.Request.Body.Close(); err != nil {
			// Log close error but don't fail the request
		}
	}()

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

	// Use Sonic for ultra-fast JSON deserialization
	if err := json.Unmarshal(body, v); err != nil {
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

	locale := c.Locale()
	if c.App != nil {
		if valSvc := c.App.Get("validator"); valSvc != nil {
			if validator, ok := valSvc.(interface {
				ValidateStruct(any, ...string) error
			}); ok {
				return validator.ValidateStruct(v, locale)
			}
		}
	}
	return Validate(v, locale)
}

// ─── Parameters ───────────────────────────────────────────────────────

// Param returns a URL path parameter value (from chi's URL params).
func (c *Context) Param(key string) string {
	return chi.URLParam(c.Request, key)
}

// Params returns the router parameters from the request URL.
func (c *Context) Params(key string) string {
	return chi.URLParam(c.Request, key)
}

// Query returns a query string parameter value.
func (c *Context) Query(key string) string {
	if c.queryCache == nil {
		c.queryCache = c.Request.URL.Query()
	}
	vals := c.queryCache[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
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

// Status returns the response status code.
func (c *Context) Status() int {
	return c.resp.Status()
}

// JSON sends a JSON response with the given data and optional status code.
// Uses Sonic JSON for ultra-fast serialization.
// Default status is 200 OK.
func (c *Context) JSON(data any, status ...int) error {
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}

	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true

	// Use Sonic for ultra-fast JSON serialization
	jsonStr, err := json.MarshalToString(data)
	if err != nil {
		return err
	}
	_, err = c.Writer.Write([]byte(jsonStr))
	return err
}

// Error sends a structured JSON error response.
// In production, it includes a unique ErrorID for log correlation.
func (c *Context) Error(code int, message string) error {
	errorID := uuid.New().String()

	// Log the error internally with the ID
	c.Logger().Error(message,
		slog.Int("status", code),
		slog.String("error_id", errorID),
	)

	resp := map[string]any{
		"error": map[string]any{
			"code":     http.StatusText(code),
			"message":  message,
			"error_id": errorID,
		},
	}

	return c.JSON(resp, code)
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

// File sends a file response.
func (c *Context) File(filepath string) error {
	http.ServeFile(c.Writer, c.Request, filepath)
	c.written = true
	return nil
}

// SetCookie sets a cookie on the response, enforcing secure defaults.
func (c *Context) SetCookie(cookie *http.Cookie) {
	if cookie.Path == "" {
		cookie.Path = "/"
	}

	// Enforce HttpOnly by default unless explicitly disabled (by setting it to false in a new cookie struct)
	// Actually, we usually want it true. If the user provided a cookie, we respect their choice but
	// we can nudge them. Let's just enforce defaults if they are zero-valued.
	cookie.HttpOnly = true

	// SameSite defaults to Lax
	if cookie.SameSite == http.SameSiteDefaultMode {
		cookie.SameSite = http.SameSiteLaxMode
	}

	// Secure flag enforcement
	isProd := false
	if c.App != nil {
		isProd = c.App.Env.IsProd()
	}

	isHTTPS := c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https"
	if isProd || isHTTPS {
		cookie.Secure = true
	}

	http.SetCookie(c.Writer, cookie)
}

// FormFile returns the uploaded file for the given form field.
func (c *Context) FormFile(field string) (*UploadedFile, error) {
	file, header, err := c.Request.FormFile(field)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log close error but don't fail the request
		}
	}()

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

// MimeType returns the detected MIME type of the file content.
// It uses http.DetectContentType which looks at the first 512 bytes.
func (f *UploadedFile) MimeType() string {
	return http.DetectContentType(f.Content)
}

// Extension returns the file extension without the leading dot.
func (f *UploadedFile) Extension() string {
	parts := strings.Split(f.Name, ".")
	if len(parts) > 1 {
		return strings.ToLower(parts[len(parts)-1])
	}
	return ""
}

// ValidateOptions represents options for validating an uploaded file.
type ValidateOptions struct {
	MaxSize           int64
	AllowedMimeTypes  []string
	AllowedExtensions []string
}

// Validate checks the file against the given options.
func (f *UploadedFile) Validate(opts ValidateOptions) error {
	if opts.MaxSize > 0 && f.Size > opts.MaxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed %d", f.Size, opts.MaxSize)
	}

	if len(opts.AllowedMimeTypes) > 0 {
		mime := f.MimeType()
		found := false
		for _, allowed := range opts.AllowedMimeTypes {
			if mime == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("mime type %s is not allowed", mime)
		}
	}

	if len(opts.AllowedExtensions) > 0 {
		ext := f.Extension()
		found := false
		for _, allowed := range opts.AllowedExtensions {
			if ext == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("extension %s is not allowed", ext)
		}
	}

	return nil
}

// Storable is an interface for storage drivers to avoid import cycles.
type Storable interface {
	Put(ctx context.Context, path string, data []byte) error
}

// Store saves the uploaded file to the given storage driver.
func (f *UploadedFile) Store(ctx context.Context, s Storable, path string) error {
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
func (c *Context) AuthUser() *auth.AuthClaims {
	claims, _ := c.Request.Context().Value(authUserKey).(*auth.AuthClaims)
	return claims
}

// IsAuthenticated returns true if the request has an authenticated user.
func (c *Context) IsAuthenticated() bool {
	return c.AuthUser() != nil
}

// SetAuthUser stores the authenticated user's claims in the request context.
func (c *Context) SetAuthUser(claims *auth.AuthClaims) {
	ctx := context.WithValue(c.Request.Context(), authUserKey, claims)
	c.Request = c.Request.WithContext(ctx)
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

// Allows is an alias for Can.
func (c *Context) Allows(action string, subject any) bool {
	return c.Can(action, subject)
}

// Denies is the inverse of Can.
func (c *Context) Denies(action string, subject any) bool {
	return !c.Can(action, subject)
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

// RealIP returns the client's real IP address, respecting proxy headers.
func (c *Context) RealIP() string {
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := c.Request.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	host, _, _ := strings.Cut(c.Request.RemoteAddr, ":")
	return host
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

// Logger returns a contextual logger for the current request.
// It includes method, path, ip, request_id, user_id, and trace_id.
func (c *Context) Logger() *slog.Logger {
	if c.App == nil || c.App.Logger == nil {
		return slog.Default()
	}

	logger := c.App.Logger.With(
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("ip", c.ClientIP()),
	)

	// Unified correlation ID propagation
	reqID := c.GetString("request_id")
	if reqID == "" {
		reqID = c.Header("X-Request-ID")
	}
	if reqID != "" {
		logger = logger.With("request_id", reqID)
	}

	if claims := c.AuthUser(); claims != nil && claims.UserID != "" {
		logger = logger.With("user_id", claims.UserID)
	}

	// Trace ID from OTEL
	spanCtx := trace.SpanContextFromContext(c.Request.Context())
	if spanCtx.HasTraceID() {
		logger = logger.With("trace_id", spanCtx.TraceID().String())
	} else {
		// Fallback to header or local ID
		traceID := c.Header("X-Trace-ID")
		if traceID == "" {
			traceID = c.GetString("trace_id")
		}
		if traceID != "" {
			logger = logger.With(slog.String("trace_id", traceID))
		}
	}

	return logger
}

// ─── Internationalization ─────────────────────────────────────────────

// Locale returns the current context's locale.
func (c *Context) Locale() string {
	locale, _ := c.Get("astra_locale").(string)
	if locale == "" {
		return "en" // Default fallback if middleware not used
	}
	return locale
}

// T translates a key in the current locale.
func (c *Context) T(key string, args ...any) string {
	if c.App != nil {
		if i18nSvc := c.App.Get("i18n"); i18nSvc != nil {
			type translator interface {
				T(locale, key string, args ...any) string
			}
			if t, ok := i18nSvc.(translator); ok {
				return t.T(c.Locale(), key, args...)
			}
		}
	}
	return key
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

// ─── Session ──────────────────────────────────────────────────────────

// Session returns the session for this request.
// Requires session.Middleware to be registered on the router.
// Returns nil if no session is in the request context.
//
// Example:
//
//	sess := c.Session()
//	if sess != nil {
//	    userID := sess.GetInt("user_id")
//	    sess.Set("last_seen", time.Now().Unix())
//	}
func (c *Context) Session() sessionGetter {
	// The session package stores sessions under the string key "astra.session".
	// We use the same string to avoid importing the session package
	// (which may import http types, risking circular dependencies).
	raw := c.Request.Context().Value("astra.session")
	if raw == nil {
		return nil
	}
	if s, ok := raw.(sessionGetter); ok {
		return s
	}
	return nil
}

// RegenerateSession rotates the session ID while preserving data.
// This is typically called after login to prevent session fixation.
func (c *Context) RegenerateSession() error {
	sess := c.Session()
	if sess != nil {
		return sess.Regenerate(c.Writer)
	}
	return nil
}

// sessionGetter is the minimum interface of *session.Session we need here.
// Using an interface avoids an import cycle (session → http would be circular).
type sessionGetter interface {
	Get(key string) any
	GetString(key string) string
	GetInt(key string) int
	Set(key string, value any)
	Delete(key string)
	Has(key string) bool
	Flash(key string) any
	Save(w http.ResponseWriter) error
	Clear()
	ID() string
	Regenerate(w http.ResponseWriter) error
}

// Throttle rate limits the frequency of occurrences for a specific key.
// Returns an error (429 Too Many Requests) if the limit is exceeded.
func (c *Context) Throttle(key string, limit int, window time.Duration) error {
	if c.App == nil {
		return nil // No app container, can't throttle
	}

	// Get Redis client from container
	var rdb goredis.UniversalClient
	if redisSvc := c.App.Get("redis"); redisSvc != nil {
		rdb = redisSvc.(goredis.UniversalClient)
	}

	if rdb == nil {
		c.Logger().Warn("Throttle: Redis not configured, skipping rate limit check")
		return nil
	}

	allowed, remaining, resetAt, err := RateLimitCheck(c, rdb, "astra:throttle:"+key, limit, window, SlidingWindow)
	if err != nil {
		return err
	}

	c.SetHeader("X-RateLimit-Limit", strconv.Itoa(limit))
	c.SetHeader("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
	c.SetHeader("X-RateLimit-Reset", strconv.FormatInt(time.UnixMilli(resetAt).Unix(), 10))

	if !allowed {
		retryAfter := int(time.Until(time.UnixMilli(resetAt)).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		c.SetHeader("Retry-After", strconv.Itoa(retryAfter))
		return NewHTTPError(http.StatusTooManyRequests, "THROTTLED", "Too many attempts, please try again later.")
	}

	return nil
}
