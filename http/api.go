package http

import (
	"net/http"
)

// ─── Standard Error Codes ─────────────────────────────────────────────

const (
	ErrCodeValidation   = "VALIDATION_ERROR"
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden    = "FORBIDDEN"
	ErrCodeConflict     = "CONFLICT"
	ErrCodeRateLimit    = "RATE_LIMIT_EXCEEDED"
	ErrCodeInternal     = "INTERNAL_ERROR"
	ErrCodeBadRequest   = "BAD_REQUEST"
)

// ─── API Envelope Types ───────────────────────────────────────────────

// APIResponse is the standard JSON envelope for successful responses.
type APIResponse struct {
	Data any            `json:"data"`
	Meta map[string]any `json:"meta,omitempty"`
}

// APIError is the standard JSON envelope for error responses.
type APIError struct {
	Error APIErrorBody `json:"error"`
}

// APIErrorBody holds the structured error fields.
type APIErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// PaginationMeta is the standard pagination metadata included in list responses.
type PaginationMeta struct {
	Total    int `json:"total"`
	Page     int `json:"page"`
	PerPage  int `json:"per_page"`
	LastPage int `json:"last_page"`
}

// CursorMeta is the metadata for cursor-based pagination responses.
type CursorMeta struct {
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// ─── Success Helpers ──────────────────────────────────────────────────

// Success sends a 200 JSON response wrapped in the standard envelope.
//
//	c.Success(user)
//	→ {"data": {...}}
func (c *Context) Success(data any) error {
	return c.JSON(APIResponse{Data: data})
}

// SuccessWithMeta sends a 200 JSON response with custom metadata.
//
//	c.SuccessWithMeta(users, map[string]any{"cached": true})
//	→ {"data": [...], "meta": {"cached": true}}
func (c *Context) SuccessWithMeta(data any, meta map[string]any) error {
	return c.JSON(APIResponse{Data: data, Meta: meta})
}

// ─── Paginated Helpers ────────────────────────────────────────────────

// PaginatedJSON sends a paginated response with standard pagination metadata.
// Works with any db.Paginated[T] result by accepting its components.
//
//	result, _ := qb.Paginate(ctx, page, perPage)
//	c.PaginatedJSON(result.Data, result.Total, result.Page, result.PerPage, result.LastPage)
func (c *Context) PaginatedJSON(data any, total, page, perPage, lastPage int) error {
	return c.JSON(APIResponse{
		Data: data,
		Meta: map[string]any{
			"pagination": PaginationMeta{
				Total:    total,
				Page:     page,
				PerPage:  perPage,
				LastPage: lastPage,
			},
		},
	})
}

// CursorJSON sends a cursor-paginated response with standard cursor metadata.
//
//	result, _ := qb.CursorPaginate(ctx, "id", cursor, limit)
//	c.CursorJSON(result.Data, result.NextCursor, result.HasMore)
func (c *Context) CursorJSON(data any, nextCursor string, hasMore bool) error {
	return c.JSON(APIResponse{
		Data: data,
		Meta: map[string]any{
			"cursor": CursorMeta{
				NextCursor: nextCursor,
				HasMore:    hasMore,
			},
		},
	})
}

// ─── Error Helpers ────────────────────────────────────────────────────

// ErrorWithDetails sends a structured error with optional extra detail fields.
//
//	c.ErrorWithDetails(409, "CONFLICT", "email taken", map[string]any{"field": "email"})
func (c *Context) ErrorWithDetails(status int, code string, message string, details map[string]any) error {
	return c.JSON(APIError{
		Error: APIErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	}, status)
}

// NotFoundError sends a 404 error for a specific resource type.
//
//	c.NotFoundError("User")
//	→ {"error": {"code": "NOT_FOUND", "message": "User not found"}}
func (c *Context) NotFoundError(resource string) error {
	return c.ErrorWithDetails(http.StatusNotFound, ErrCodeNotFound, resource+" not found", nil)
}

// ConflictError sends a 409 error for resource conflicts.
//
//	c.ConflictError("email already exists")
func (c *Context) ConflictError(message string) error {
	return c.ErrorWithDetails(http.StatusConflict, ErrCodeConflict, message, nil)
}

// BadRequestError sends a 400 error for malformed requests.
func (c *Context) BadRequestError(message string) error {
	return c.ErrorWithDetails(http.StatusBadRequest, ErrCodeBadRequest, message, nil)
}

// UnauthorizedError sends a 401 error.
func (c *Context) UnauthorizedError(message string) error {
	if message == "" {
		message = "authentication required"
	}
	return c.ErrorWithDetails(http.StatusUnauthorized, ErrCodeUnauthorized, message, nil)
}

// ForbiddenError sends a 403 error.
func (c *Context) ForbiddenError(message string) error {
	if message == "" {
		message = "access denied"
	}
	return c.ErrorWithDetails(http.StatusForbidden, ErrCodeForbidden, message, nil)
}

// InternalError sends a 500 error.
func (c *Context) InternalError(message string) error {
	if message == "" {
		message = "an unexpected error occurred"
	}
	return c.ErrorWithDetails(http.StatusInternalServerError, ErrCodeInternal, message, nil)
}
