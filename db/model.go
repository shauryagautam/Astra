package db

import (
	"time"
)

// Model is the base model struct that can be embedded in application models.
// It provides the standard ID, CreatedAt, UpdatedAt, and DeletedAt fields.
type Model struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
