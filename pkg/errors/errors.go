package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	// General errors
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeBadRequest   ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeValidation   ErrorCode = "VALIDATION_ERROR"
	ErrCodeRateLimit    ErrorCode = "RATE_LIMIT"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeUnavailable  ErrorCode = "SERVICE_UNAVAILABLE"

	// Authentication errors
	ErrCodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrCodeTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid       ErrorCode = "TOKEN_INVALID"
	ErrCodeAccountLocked      ErrorCode = "ACCOUNT_LOCKED"
	ErrCodeAccountDisabled    ErrorCode = "ACCOUNT_DISABLED"

	// Authorization errors
	ErrCodeInsufficientPermissions ErrorCode = "INSUFFICIENT_PERMISSIONS"
	ErrCodeRoleRequired            ErrorCode = "ROLE_REQUIRED"
	ErrCodeScopeRequired           ErrorCode = "SCOPE_REQUIRED"

	// Business logic errors
	ErrCodeUserNotFound     ErrorCode = "USER_NOT_FOUND"
	ErrCodeUserExists       ErrorCode = "USER_EXISTS"
	ErrCodeTenantNotFound   ErrorCode = "TENANT_NOT_FOUND"
	ErrCodeTenantExists     ErrorCode = "TENANT_EXISTS"
	ErrCodeResourceNotFound ErrorCode = "RESOURCE_NOT_FOUND"
	ErrCodeResourceExists   ErrorCode = "RESOURCE_EXISTS"
	ErrCodeLimitExceeded    ErrorCode = "LIMIT_EXCEEDED"
	ErrCodeQuotaExceeded    ErrorCode = "QUOTA_EXCEEDED"

	// Database errors
	ErrCodeDatabaseConnection ErrorCode = "DATABASE_CONNECTION"
	ErrCodeDatabaseTimeout    ErrorCode = "DATABASE_TIMEOUT"
	ErrCodeDatabaseConstraint ErrorCode = "DATABASE_CONSTRAINT"
	ErrCodeDatabaseMigration  ErrorCode = "DATABASE_MIGRATION"

	// External service errors
	ErrCodeExternalService     ErrorCode = "EXTERNAL_SERVICE_ERROR"
	ErrCodePaymentRequired     ErrorCode = "PAYMENT_REQUIRED"
	ErrCodeSubscriptionExpired ErrorCode = "SUBSCRIPTION_EXPIRED"

	// File/Upload errors
	ErrCodeFileNotFound    ErrorCode = "FILE_NOT_FOUND"
	ErrCodeFileTooLarge    ErrorCode = "FILE_TOO_LARGE"
	ErrCodeFileTypeInvalid ErrorCode = "FILE_TYPE_INVALID"
	ErrCodeUploadFailed    ErrorCode = "UPLOAD_FAILED"
	ErrCodeStorageQuota    ErrorCode = "STORAGE_QUOTA_EXCEEDED"

	// Validation errors
	ErrCodeRequiredField ErrorCode = "REQUIRED_FIELD"
	ErrCodeInvalidFormat ErrorCode = "INVALID_FORMAT"
	ErrCodeInvalidLength ErrorCode = "INVALID_LENGTH"
	ErrCodeInvalidRange  ErrorCode = "INVALID_RANGE"
	ErrCodeInvalidEmail  ErrorCode = "INVALID_EMAIL"
	ErrCodeInvalidURL    ErrorCode = "INVALID_URL"
)

// ErrorSeverity represents error severity levels
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// Error represents a structured error
type Error struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Severity   ErrorSeverity          `json:"severity"`
	Timestamp  time.Time              `json:"timestamp"`
	RequestID  string                 `json:"request_id,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	TenantID   string                 `json:"tenant_id,omitempty"`
	StackTrace []string               `json:"stack_trace,omitempty"`
	Cause      error                  `json:"-"`
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}

// Unwrap returns the underlying cause
func (e *Error) Unwrap() error {
	return e.Cause
}

// WithDetail adds a detail to the error
func (e *Error) WithDetail(key string, value interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithRequestID sets the request ID
func (e *Error) WithRequestID(requestID string) *Error {
	e.RequestID = requestID
	return e
}

// WithUserID sets the user ID
func (e *Error) WithUserID(userID string) *Error {
	e.UserID = userID
	return e
}

// WithTenantID sets the tenant ID
func (e *Error) WithTenantID(tenantID string) *Error {
	e.TenantID = tenantID
	return e
}

// WithCause sets the underlying cause
func (e *Error) WithCause(cause error) *Error {
	e.Cause = cause
	return e
}

// WithStackTrace adds stack trace
func (e *Error) WithStackTrace() *Error {
	e.StackTrace = getStackTrace()
	return e
}

// HTTPStatus returns appropriate HTTP status code
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case ErrCodeBadRequest, ErrCodeValidation, ErrCodeRequiredField, ErrCodeInvalidFormat,
		ErrCodeInvalidLength, ErrCodeInvalidRange, ErrCodeInvalidEmail, ErrCodeInvalidURL:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidCredentials, ErrCodeTokenExpired, ErrCodeTokenInvalid:
		return http.StatusUnauthorized
	case ErrCodeForbidden, ErrCodeInsufficientPermissions, ErrCodeRoleRequired, ErrCodeScopeRequired:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeUserNotFound, ErrCodeTenantNotFound, ErrCodeResourceNotFound, ErrCodeFileNotFound:
		return http.StatusNotFound
	case ErrCodeConflict, ErrCodeUserExists, ErrCodeTenantExists, ErrCodeResourceExists:
		return http.StatusConflict
	case ErrCodeRateLimit:
		return http.StatusTooManyRequests
	case ErrCodePaymentRequired:
		return http.StatusPaymentRequired
	case ErrCodeUnavailable, ErrCodeDatabaseConnection:
		return http.StatusServiceUnavailable
	case ErrCodeTimeout, ErrCodeDatabaseTimeout:
		return http.StatusRequestTimeout
	case ErrCodeFileTooLarge:
		return http.StatusRequestEntityTooLarge
	case ErrCodeStorageQuota, ErrCodeLimitExceeded, ErrCodeQuotaExceeded:
		return http.StatusInsufficientStorage
	default:
		return http.StatusInternalServerError
	}
}

// New creates a new error
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
	}
}

// Newf creates a new error with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *Error {
	return &Error{
		Code:      code,
		Message:   fmt.Sprintf(format, args...),
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
	}
}

// Internal creates an internal server error
func Internal(message string) *Error {
	return &Error{
		Code:      ErrCodeInternal,
		Message:   message,
		Severity:  SeverityHigh,
		Timestamp: time.Now(),
	}
}

// Internalf creates an internal server error with formatted message
func Internalf(format string, args ...interface{}) *Error {
	return &Error{
		Code:      ErrCodeInternal,
		Message:   fmt.Sprintf(format, args...),
		Severity:  SeverityHigh,
		Timestamp: time.Now(),
	}
}

// BadRequest creates a bad request error
func BadRequest(message string) *Error {
	return &Error{
		Code:      ErrCodeBadRequest,
		Message:   message,
		Severity:  SeverityLow,
		Timestamp: time.Now(),
	}
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *Error {
	return &Error{
		Code:      ErrCodeUnauthorized,
		Message:   message,
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
	}
}

// Forbidden creates a forbidden error
func Forbidden(message string) *Error {
	return &Error{
		Code:      ErrCodeForbidden,
		Message:   message,
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
	}
}

// NotFound creates a not found error
func NotFound(message string) *Error {
	return &Error{
		Code:      ErrCodeNotFound,
		Message:   message,
		Severity:  SeverityLow,
		Timestamp: time.Now(),
	}
}

// Validation creates a validation error
func Validation(message string) *Error {
	return &Error{
		Code:      ErrCodeValidation,
		Message:   message,
		Severity:  SeverityLow,
		Timestamp: time.Now(),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code ErrorCode, message string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
		Cause:     err,
	}
}

// Wrapf wraps an existing error with formatted message
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *Error {
	return &Error{
		Code:      code,
		Message:   fmt.Sprintf(format, args...),
		Severity:  SeverityMedium,
		Timestamp: time.Now(),
		Cause:     err,
	}
}

// Is checks if error matches specific error code
func Is(err error, code ErrorCode) bool {
	if astraErr, ok := err.(*Error); ok {
		return astraErr.Code == code
	}
	return false
}

// GetCode extracts error code from error
func GetCode(err error) ErrorCode {
	if astraErr, ok := err.(*Error); ok {
		return astraErr.Code
	}
	return ErrCodeInternal
}

// GetSeverity extracts error severity from error
func GetSeverity(err error) ErrorSeverity {
	if astraErr, ok := err.(*Error); ok {
		return astraErr.Severity
	}
	return SeverityMedium
}

// ErrorHandler provides centralized error handling
type ErrorHandler struct {
	logger Logger
	config HandlerConfig
}

// HandlerConfig represents error handler configuration
type HandlerConfig struct {
	IncludeStackTrace bool          `json:"include_stack_trace"`
	IncludeDetails    bool          `json:"include_details"`
	LogLevel          ErrorSeverity `json:"log_level"`
}

// Logger interface for error logging
type Logger interface {
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger Logger, config HandlerConfig) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
		config: config,
	}
}

// Handle handles an error and returns appropriate HTTP response
func (eh *ErrorHandler) Handle(err error, requestID, userID, tenantID string) *ErrorResponse {
	astraErr, ok := err.(*Error)
	if !ok {
		// Convert regular error to Astra error
		astraErr = Internal(err.Error()).WithCause(err)
	}

	// Add context
	if requestID != "" {
		astraErr.WithRequestID(requestID)
	}
	if userID != "" {
		astraErr.WithUserID(userID)
	}
	if tenantID != "" {
		astraErr.WithTenantID(tenantID)
	}

	// Log error based on severity
	eh.logError(astraErr)

	// Create response
	response := &ErrorResponse{
		Error: ErrorInfo{
			Code:      string(astraErr.Code),
			Message:   astraErr.Message,
			Timestamp: astraErr.Timestamp,
		},
		Status: astraErr.HTTPStatus(),
	}

	// Include request ID if available
	if astraErr.RequestID != "" {
		response.RequestID = astraErr.RequestID
	}

	// Include details in development or if configured
	if eh.config.IncludeDetails && astraErr.Details != nil {
		response.Error.Details = astraErr.Details
	}

	// Include stack trace in development or if configured
	if eh.config.IncludeStackTrace && astraErr.StackTrace != nil {
		response.Error.StackTrace = astraErr.StackTrace
	}

	return response
}

// logError logs error based on severity
func (eh *ErrorHandler) logError(err *Error) {
	fields := []interface{}{
		"code", err.Code,
		"severity", err.Severity,
		"timestamp", err.Timestamp,
	}

	if err.RequestID != "" {
		fields = append(fields, "request_id", err.RequestID)
	}
	if err.UserID != "" {
		fields = append(fields, "user_id", err.UserID)
	}
	if err.TenantID != "" {
		fields = append(fields, "tenant_id", err.TenantID)
	}
	if err.Details != nil {
		fields = append(fields, "details", err.Details)
	}

	switch err.Severity {
	case SeverityCritical, SeverityHigh:
		eh.logger.Error(err.Message, fields...)
	case SeverityMedium:
		eh.logger.Warn(err.Message, fields...)
	case SeverityLow:
		eh.logger.Info(err.Message, fields...)
	}

	// Always log stack trace for critical errors
	if err.Severity == SeverityCritical && err.StackTrace != nil {
		eh.logger.Error("Stack trace", "stack", strings.Join(err.StackTrace, "\n"))
	}
}

// ErrorResponse represents HTTP error response
type ErrorResponse struct {
	Error     ErrorInfo `json:"error"`
	Status    int       `json:"status"`
	RequestID string    `json:"request_id,omitempty"`
}

// ErrorInfo represents error information in response
type ErrorInfo struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	StackTrace []string               `json:"stack_trace,omitempty"`
}

// JSON returns JSON representation of error response
func (er *ErrorResponse) JSON() ([]byte, error) {
	return json.Marshal(er)
}

// getStackTrace captures current stack trace
func getStackTrace() []string {
	var stack []string
	for i := 2; i < 15; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		frame := fmt.Sprintf("%s:%d %s", filepath.Base(file), line, fn.Name())
		stack = append(stack, frame)
	}

	return stack
}

// ValidationError represents multiple validation errors
type ValidationError struct {
	Errors map[string]string `json:"errors"`
}

// Error implements error interface
func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", ve.Errors)
}

// NewValidationError creates a validation error
func NewValidationError(errors map[string]string) *ValidationError {
	return &ValidationError{
		Errors: errors,
	}
}

// Add adds a validation error
func (ve *ValidationError) Add(field, message string) {
	if ve.Errors == nil {
		ve.Errors = make(map[string]string)
	}
	ve.Errors[field] = message
}

// Has checks if there are any validation errors
func (ve *ValidationError) Has() bool {
	return len(ve.Errors) > 0
}

// ToAstraError converts to Astra error
func (ve *ValidationError) ToAstraError() *Error {
	return Validation("Validation failed").WithDetail("validation_errors", ve.Errors)
}

// RecoveryMiddleware provides panic recovery
type RecoveryMiddleware struct {
	errorHandler *ErrorHandler
	logger       Logger
}

// NewRecoveryMiddleware creates new recovery middleware
func NewRecoveryMiddleware(errorHandler *ErrorHandler, logger Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		errorHandler: errorHandler,
		logger:       logger,
	}
}

// Recover recovers from panic and converts to error
func (rm *RecoveryMiddleware) Recover(requestID, userID, tenantID string) error {
	if r := recover(); r != nil {
		var err error
		switch v := r.(type) {
		case error:
			err = v
		case string:
			err = fmt.Errorf("%s", v)
		default:
			err = fmt.Errorf("panic: %v", v)
		}

		rm.logger.Error("Panic recovered",
			"error", err,
			"request_id", requestID,
			"user_id", userID,
			"tenant_id", tenantID,
		)

		return Internal("Internal server error").
			WithCause(err).
			WithStackTrace().
			WithRequestID(requestID).
			WithUserID(userID).
			WithTenantID(tenantID)
	}
	return nil
}
