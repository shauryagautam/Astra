// Package exceptions provides centralized error handling for Astra Go.
// Mirrors Astra's ExceptionHandler in app/Exceptions/Handler.ts.
//
// Usage:
//
//	handler := exceptions.NewHandler(true) // debug mode
//	handler.Handle(ctx, err)
//
// Custom HttpExceptions:
//
//	return exceptions.NewHttpException(422, "Validation failed", validationErrors)
//	return exceptions.NotFound("User not found")
//	return exceptions.Unauthorized("Invalid credentials")
package exceptions

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/shaurya/astra/contracts"
)

// HttpException is an error with an HTTP status code and optional data.
// Use this to return structured error responses from handlers.
type HttpException struct {
	// StatusCode is the HTTP status code.
	StatusCode int `json:"status"`

	// Code is an optional application-specific error code.
	Code string `json:"code,omitempty"`

	// Message is the error message.
	Message string `json:"message"`

	// Data holds optional payload (e.g., validation errors).
	Data any `json:"data,omitempty"`

	// Internal holds the original error (not serialized).
	Internal error `json:"-"`
}

// Error implements the error interface.
func (e *HttpException) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

// Unwrap returns the internal error.
func (e *HttpException) Unwrap() error {
	return e.Internal
}

// NewHttpException creates a new HttpException.
func NewHttpException(status int, message string, data ...any) *HttpException {
	e := &HttpException{
		StatusCode: status,
		Message:    message,
	}
	if len(data) > 0 {
		e.Data = data[0]
	}
	return e
}

// ── Convenience Constructors ──────────────────────────────────────────

// BadRequest creates a 400 Bad Request error.
func BadRequest(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusBadRequest, message, data...)
}

// Unauthorized creates a 401 Unauthorized error.
func Unauthorized(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusUnauthorized, message, data...)
}

// Forbidden creates a 403 Forbidden error.
func Forbidden(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusForbidden, message, data...)
}

// NotFound creates a 404 Not Found error.
func NotFound(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusNotFound, message, data...)
}

// Conflict creates a 409 Conflict error.
func Conflict(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusConflict, message, data...)
}

// UnprocessableEntity creates a 422 Unprocessable Entity error (validation).
func UnprocessableEntity(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusUnprocessableEntity, message, data...)
}

// TooManyRequests creates a 429 Too Many Requests error.
func TooManyRequests(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusTooManyRequests, message, data...)
}

// InternalServerError creates a 500 Internal Server Error.
func InternalServerError(message string, data ...any) *HttpException {
	return NewHttpException(http.StatusInternalServerError, message, data...)
}

// ══════════════════════════════════════════════════════════════════════
// Exception Handler
// ══════════════════════════════════════════════════════════════════════

// Handler is the centralized exception handler.
// It processes errors from route handlers and sends appropriate responses.
type Handler struct {
	debug  bool
	logger *log.Logger
}

// NewHandler creates a new exception handler.
// When debug is true, stack traces and internal errors are included in responses.
func NewHandler(debug bool) *Handler {
	return &Handler{
		debug:  debug,
		logger: log.New(os.Stderr, "[astra:error] ", log.LstdFlags),
	}
}

// Handle processes an error and sends an appropriate HTTP response.
func (h *Handler) Handle(ctx contracts.HttpContextContract, err error) {
	if ctx.Response().IsCommitted() {
		return
	}

	// Report the error (logging)
	h.Report(err)

	// Determine the response based on error type
	switch e := err.(type) {
	case *HttpException:
		h.handleHttpException(ctx, e)
	default:
		h.handleGenericError(ctx, err)
	}
}

// Report logs the error.
func (h *Handler) Report(err error) {
	switch e := err.(type) {
	case *HttpException:
		// Don't log 4xx client errors as server errors
		if e.StatusCode >= 500 {
			h.logger.Printf("ERROR [%d] %s", e.StatusCode, e.Error())
			if h.debug {
				h.logStackTrace()
			}
		}
	default:
		h.logger.Printf("ERROR %v", err)
		if h.debug {
			h.logStackTrace()
		}
	}
}

// handleHttpException handles a typed HttpException.
func (h *Handler) handleHttpException(ctx contracts.HttpContextContract, e *HttpException) {
	response := map[string]any{
		"error":   http.StatusText(e.StatusCode),
		"message": e.Message,
		"status":  e.StatusCode,
	}

	if e.Code != "" {
		response["code"] = e.Code
	}

	if e.Data != nil {
		response["errors"] = e.Data
	}

	if h.debug && e.Internal != nil {
		response["internal"] = e.Internal.Error()
	}

	ctx.Response().Status(e.StatusCode).Json(response) //nolint:errcheck
}

// handleGenericError handles an unknown error type.
func (h *Handler) handleGenericError(ctx contracts.HttpContextContract, err error) {
	response := map[string]any{
		"error":   "Internal Server Error",
		"message": "An unexpected error occurred",
		"status":  500,
	}

	if h.debug {
		response["message"] = err.Error()
		response["stack"] = getStackTrace()
	}

	ctx.Response().Status(http.StatusInternalServerError).Json(response) //nolint:errcheck
}

// logStackTrace logs the current stack trace.
func (h *Handler) logStackTrace() {
	stack := getStackTrace()
	h.logger.Printf("Stack trace:\n%s", stack)
}

// getStackTrace returns the current goroutine's stack trace.
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// Ensure Handler implements ExceptionHandlerContract.
var _ contracts.ExceptionHandlerContract = (*Handler)(nil)
