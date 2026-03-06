package db

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Model is the base model struct that can be embedded in application models.
// It uses UUIDs for primary keys instead of auto-incrementing integers,
// which is standard for distributed systems.
type Model struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// JSONMap is an alias for Gorm's JSON datatype, native JSONB in Postgres.
type JSONMap = datatypes.JSONMap

// JSON is an alias for Gorm's generic JSON data structure.
type JSON = datatypes.JSON

// Date is an alias for Gorm's Date type that strictly saves DATE ignoring times.
type Date = datatypes.Date

// BeforeCreate is a GORM hook that auto-generates a UUID if none was provided.
func (m *Model) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
