package http

import (
	"fmt"
	"net/http"
)

// ─── Sentinel Errors ──────────────────────────────────────────────────

// Common HTTP errors used throughout the framework.
var (
	ErrNotFound     = NewHTTPError(http.StatusNotFound, "NOT_FOUND", "resource not found")
	ErrUnauthorized = NewHTTPError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	ErrForbidden    = NewHTTPError(http.StatusForbidden, "FORBIDDEN", "access denied")
	ErrConflict     = NewHTTPError(http.StatusConflict, "CONFLICT", "resource conflict")
)

// HTTPError represents a structured HTTP error with a status code, error
// code string, and human-readable message.
type HTTPError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewHTTPError creates a new HTTPError.
func NewHTTPError(status int, code string, message string) *HTTPError {
	return &HTTPError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.Status, e.Code, e.Message)
}

// WithMessage returns a copy of the error with a different message.
func (e *HTTPError) WithMessage(msg string) *HTTPError {
	return &HTTPError{
		Status:  e.Status,
		Code:    e.Code,
		Message: msg,
	}
}
