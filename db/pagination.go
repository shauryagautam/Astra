package db

// Paginated represents a paginated result set with offset-based pagination.
type Paginated[T any] struct {
	Data     []T `json:"data"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PerPage  int `json:"per_page"`
	LastPage int `json:"last_page"`
}

// CursorPaginated represents a cursor-based paginated result set.
// Ideal for mobile clients and infinite scroll — avoids the N+offset performance issue.
type CursorPaginated[T any] struct {
	Data       []T    `json:"data"`
	NextCursor string `json:"next_cursor"` // opaque cursor for next page; empty if no more pages
	HasMore    bool   `json:"has_more"`
}
