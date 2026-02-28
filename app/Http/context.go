package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/shaurya/adonis/contracts"
)

// Request wraps the standard http.Request with convenience methods.
// Mirrors AdonisJS's Request class.
type Request struct {
	raw       *http.Request
	body      []byte
	bodyRead  bool
	allMerged map[string]any
}

// NewRequest creates a Request wrapping the given http.Request.
func NewRequest(r *http.Request) *Request {
	return &Request{raw: r}
}

func (r *Request) Method() string {
	return r.raw.Method
}

func (r *Request) URL() string {
	return r.raw.URL.Path
}

func (r *Request) Header(key string) string {
	return r.raw.Header.Get(key)
}

func (r *Request) Headers() http.Header {
	return r.raw.Header
}

func (r *Request) Input(key string) string {
	return r.InputOr(key, "")
}

func (r *Request) InputOr(key string, defaultValue string) string {
	all := r.All()
	if val, ok := all[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultValue
}

// All returns all input data (merged query + body).
func (r *Request) All() map[string]any {
	if r.allMerged != nil {
		return r.allMerged
	}

	result := make(map[string]any)

	// Query params
	for key, values := range r.raw.URL.Query() {
		if len(values) == 1 {
			result[key] = values[0]
		} else {
			result[key] = values
		}
	}

	// Form data
	if r.raw.Method == http.MethodPost || r.raw.Method == http.MethodPut || r.raw.Method == http.MethodPatch {
		contentType := r.raw.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/json") {
			body, err := r.Body()
			if err == nil {
				var jsonData map[string]any
				if err := json.Unmarshal(body, &jsonData); err == nil {
					for k, v := range jsonData {
						result[k] = v
					}
				}
			}
		} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") ||
			strings.HasPrefix(contentType, "multipart/form-data") {
			_ = r.raw.ParseForm()
			for key, values := range r.raw.PostForm {
				if len(values) == 1 {
					result[key] = values[0]
				} else {
					result[key] = values
				}
			}
		}
	}

	r.allMerged = result
	return result
}

func (r *Request) Only(keys ...string) map[string]any {
	all := r.All()
	result := make(map[string]any, len(keys))
	for _, key := range keys {
		if val, ok := all[key]; ok {
			result[key] = val
		}
	}
	return result
}

func (r *Request) Except(keys ...string) map[string]any {
	all := r.All()
	exclude := make(map[string]bool, len(keys))
	for _, key := range keys {
		exclude[key] = true
	}
	result := make(map[string]any)
	for k, v := range all {
		if !exclude[k] {
			result[k] = v
		}
	}
	return result
}

func (r *Request) Qs() map[string]string {
	result := make(map[string]string)
	for key, values := range r.raw.URL.Query() {
		result[key] = values[0]
	}
	return result
}

func (r *Request) Cookie(name string) string {
	c, err := r.raw.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func (r *Request) HasBody() bool {
	return r.raw.Body != nil && r.raw.ContentLength != 0
}

func (r *Request) IP() string {
	// Check X-Forwarded-For first
	if xff := r.raw.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Check X-Real-Ip
	if xri := r.raw.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	host, _, _ := strings.Cut(r.raw.RemoteAddr, ":")
	return host
}

func (r *Request) IsAjax() bool {
	return r.raw.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

func (r *Request) Raw() *http.Request {
	return r.raw
}

func (r *Request) Body() ([]byte, error) {
	if r.bodyRead {
		return r.body, nil
	}
	body, err := io.ReadAll(r.raw.Body)
	if err != nil {
		return nil, err
	}
	r.body = body
	r.bodyRead = true
	return body, nil
}

// Validate parses the request body into the given struct.
// This is a convenience method for JSON APIs.
func (r *Request) Validate(dest any) error {
	body, err := r.Body()
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

// Ensure Request implements RequestContract at compile time.
var _ contracts.RequestContract = (*Request)(nil)

// Response wraps http.ResponseWriter with convenience methods.
// Mirrors AdonisJS's Response class.
type Response struct {
	writer     http.ResponseWriter
	statusCode int
	committed  bool
}

// NewResponse creates a Response wrapping the given http.ResponseWriter.
func NewResponse(w http.ResponseWriter) *Response {
	return &Response{
		writer:     w,
		statusCode: http.StatusOK,
	}
}

func (r *Response) Status(code int) contracts.ResponseContract {
	r.statusCode = code
	return r
}

func (r *Response) Json(data any) error {
	r.writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	r.writer.WriteHeader(r.statusCode)
	r.committed = true
	return json.NewEncoder(r.writer).Encode(data)
}

func (r *Response) Send(data string) error {
	r.writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	r.writer.WriteHeader(r.statusCode)
	r.committed = true
	_, err := r.writer.Write([]byte(data))
	return err
}

func (r *Response) SendBytes(data []byte) error {
	r.writer.WriteHeader(r.statusCode)
	r.committed = true
	_, err := r.writer.Write(data)
	return err
}

func (r *Response) Header(key string, value string) contracts.ResponseContract {
	r.writer.Header().Set(key, value)
	return r
}

func (r *Response) Cookie(name string, value string, options ...contracts.CookieOption) contracts.ResponseContract {
	cookie := &http.Cookie{
		Name:  name,
		Value: url.QueryEscape(value),
		Path:  "/",
	}
	if len(options) > 0 {
		opt := options[0]
		if opt.MaxAge != 0 {
			cookie.MaxAge = opt.MaxAge
		}
		if opt.Path != "" {
			cookie.Path = opt.Path
		}
		if opt.Domain != "" {
			cookie.Domain = opt.Domain
		}
		cookie.Secure = opt.Secure
		cookie.HttpOnly = opt.HttpOnly
		cookie.SameSite = opt.SameSite
	}
	http.SetCookie(r.writer, cookie)
	return r
}

func (r *Response) ClearCookie(name string) contracts.ResponseContract {
	http.SetCookie(r.writer, &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	return r
}

func (r *Response) Redirect(url string) error {
	return r.RedirectStatus(url, http.StatusFound)
}

func (r *Response) RedirectStatus(url string, code int) error {
	r.writer.Header().Set("Location", url)
	r.writer.WriteHeader(code)
	r.committed = true
	return nil
}

func (r *Response) Abort(code int, message string) error {
	r.statusCode = code
	return r.Json(map[string]any{
		"error":   http.StatusText(code),
		"message": message,
		"status":  code,
	})
}

func (r *Response) NoContent() error {
	r.writer.WriteHeader(http.StatusNoContent)
	r.committed = true
	return nil
}

func (r *Response) Created(data any) error {
	r.statusCode = http.StatusCreated
	return r.Json(data)
}

func (r *Response) GetStatusCode() int {
	return r.statusCode
}

func (r *Response) IsCommitted() bool {
	return r.committed
}

func (r *Response) Raw() http.ResponseWriter {
	return r.writer
}

// Ensure Response implements ResponseContract at compile time.
var _ contracts.ResponseContract = (*Response)(nil)

// HttpContext is the central context object passed to all handlers and middleware.
// Mirrors AdonisJS's HttpContext.
type HttpContext struct {
	request  *Request
	response *Response
	params   map[string]string
	values   map[string]any
	rawReq   *http.Request
	rawRes   http.ResponseWriter
}

// NewHttpContext creates a new HttpContext for the given request/response pair.
func NewHttpContext(w http.ResponseWriter, r *http.Request) *HttpContext {
	return &HttpContext{
		request:  NewRequest(r),
		response: NewResponse(w),
		params:   make(map[string]string),
		values:   make(map[string]any),
		rawReq:   r,
		rawRes:   w,
	}
}

func (ctx *HttpContext) Request() contracts.RequestContract {
	return ctx.request
}

func (ctx *HttpContext) Response() contracts.ResponseContract {
	return ctx.response
}

func (ctx *HttpContext) Params() map[string]string {
	return ctx.params
}

func (ctx *HttpContext) Param(key string) string {
	return ctx.params[key]
}

// SetParams sets the route parameters (called by the router after matching).
func (ctx *HttpContext) SetParams(params map[string]string) {
	ctx.params = params
}

func (ctx *HttpContext) Logger() any {
	return nil // Will be wired to container logger
}

func (ctx *HttpContext) Auth() any {
	return ctx.GetValue("auth")
}

func (ctx *HttpContext) GetRaw() *http.Request {
	return ctx.rawReq
}

func (ctx *HttpContext) GetResponseWriter() http.ResponseWriter {
	return ctx.rawRes
}

func (ctx *HttpContext) Context() context.Context {
	return ctx.rawReq.Context()
}

func (ctx *HttpContext) WithValue(key string, value any) {
	ctx.values[key] = value
}

func (ctx *HttpContext) GetValue(key string) any {
	return ctx.values[key]
}

// Ensure HttpContext implements HttpContextContract at compile time.
var _ contracts.HttpContextContract = (*HttpContext)(nil)
