package orm

// PaginationResult holds the results of a paginated query (AdonisJS-inspired naming)
type PaginationResult[T any] struct {
	Data        []T               `json:"data"`
	Total       int64             `json:"total"`
	PerPage     int               `json:"per_page"`
	CurrentPage int               `json:"current_page"`
	LastPage    int               `json:"last_page"`
	From        int               `json:"from"`
	To          int               `json:"to"`
	Links       map[string]string `json:"links,omitempty"`
}

// Paginated represents a paginated result set (Legacy compatibility naming)
type Paginated[T any] struct {
	Data     []T `json:"data"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PerPage  int `json:"per_page"`
	LastPage int `json:"last_page"`
}

// CursorPaginated represents a cursor-based paginated result set
type CursorPaginated[T any] struct {
	Data       []T    `json:"data"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}
