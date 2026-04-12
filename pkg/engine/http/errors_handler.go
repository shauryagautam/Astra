package http

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/shauryagautam/Astra/pkg/engine/config"
)

// InteractiveErrorHandler renders rich debug error pages in development and
// structured JSON / minimal HTML in production.
type InteractiveErrorHandler struct {
	cfg     *config.AstraConfig
	env     *config.Config
	logger  *slog.Logger
	tmpl    *template.Template
	appVer  string
}

// NewInteractiveErrorHandler creates an InteractiveErrorHandler with explicit dependencies.
func NewInteractiveErrorHandler(cfg *config.AstraConfig, env *config.Config, logger *slog.Logger) *InteractiveErrorHandler {
	h := &InteractiveErrorHandler{
		cfg:    cfg,
		env:    env,
		logger: logger,
	}
	if cfg != nil {
		h.appVer = cfg.App.Version
	}
	tmpl, err := template.New("error").Parse("")
	if err == nil {
		h.tmpl = tmpl
	}
	return h
}

// Handle is the error handler function compatible with Router.errorHandler.
func (h *InteractiveErrorHandler) Handle(c *Context, err error) {
	if err == nil {
		return
	}

	isDev := h.env != nil && h.env.IsDev()
	isAPI := isAPIRequest(c.Request)

	var statusCode int
	var message string

	if httpErr, ok := err.(*HTTPError); ok {
		statusCode = httpErr.Status
		message = httpErr.Message
	} else {
		statusCode = http.StatusInternalServerError
		message = err.Error()
	}

	// Use debug.Stack() correctly
	var stackStr string
	if statusCode >= 500 {
		stackStr = string(debug.Stack())
	}

	if isAPI {
		// Structured JSON error for API routes.
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(statusCode)
		errCode := http.StatusText(statusCode)
		if errCode == "" {
			errCode = "INTERNAL_SERVER_ERROR"
		}
		
		resp := map[string]any{
			"error": map[string]any{
				"code":    strings.ToUpper(strings.ReplaceAll(errCode, " ", "_")),
				"message": message,
			},
		}
		
		if isDev && stackStr != "" {
			resp["debug"] = map[string]any{
				"stack": stackStr,
			}
		}

		_ = c.JSON(resp, statusCode)
		return
	}

	// SSR / browser route.
	if !isDev || h.tmpl == nil {
		// Minimal static 500 page for production.
		c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		c.Writer.WriteHeader(statusCode)
		_, _ = c.Writer.Write([]byte(minimalErrorPage(statusCode)))
		return
	}

	// Development: rich interactive error page.
	data := errorPageData{
		Error: errorDetails{
			Code:    statusCode,
			Message: message,
			Type:    errorType(statusCode),
			Stack:   stackStr,
		},
		Request: requestInfo{
			Method:    c.Request.Method,
			URL:       c.Request.URL.String(),
			Headers:   flatHeaders(c.Request),
			Query:     flatQuery(c.Request),
			IP:        c.ClientIP(),
			UserAgent: c.Request.Header.Get("User-Agent"),
		},
		Timestamp:  time.Now(),
		AppVersion: h.appVer,
	}

	var buf bytes.Buffer
	if err2 := h.tmpl.Execute(&buf, data); err2 != nil {
		c.Writer.Header().Set("Content-Type", "text/plain")
		c.Writer.WriteHeader(statusCode)
		_, _ = c.Writer.Write([]byte(fmt.Sprintf("Error rendering error page: %v\nOriginal error: %s", err2, message)))
		return
	}

	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(statusCode)
	_, _ = c.Writer.Write(buf.Bytes())
}

// isAPIRequest returns true when the request looks like an API call.
func isAPIRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.HasPrefix(accept, "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// minimalErrorPage returns a minimal static HTML error page for production.
func minimalErrorPage(code int) string {
	statusText := http.StatusText(code)
	if statusText == "" {
		statusText = "Internal Server Error"
	}
	return `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>` +
		statusText +
		`</title></head><body style="font-family:sans-serif;text-align:center;padding:60px;background:#f8fafc;color:#1e293b"><h1>` +
		fmt.Sprintf("%d %s", code, statusText) +
		`</h1><p>Something went wrong on our end. Please try again later.</p></body></html>`
}

// errorType returns a string category for an HTTP status code.
func errorType(code int) string {
	switch {
	case code >= 400 && code < 500:
		return "client_error"
	case code >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

// flatHeaders returns request headers as a flat string map.
func flatHeaders(r *http.Request) map[string]string {
	out := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		out[k] = strings.Join(v, ", ")
	}
	return out
}

// flatQuery returns URL query parameters as a flat string map.
func flatQuery(r *http.Request) map[string]string {
	out := make(map[string]string)
	for k, v := range r.URL.Query() {
		out[k] = strings.Join(v, ", ")
	}
	return out
}

// errorPageData is the template data for the interactive error page.
type errorPageData struct {
	Error      errorDetails
	Request    requestInfo
	Timestamp  time.Time
	AppVersion string
}

type errorDetails struct {
	Code    int
	Message string
	Type    string
	Stack   string
}

type requestInfo struct {
	Method    string
	URL       string
	Headers   map[string]string
	Query     map[string]string
	IP        string
	UserAgent string
}
