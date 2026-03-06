package db

import (
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// JSONQuery provides chainable Postgres JSONB query scopes for Gorm.
// Allows syntax like: db.Scopes(db.JSONContains("metadata", "role", "admin"))
func JSONContains(column string, key string, value string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(datatypes.JSONQuery(column).HasKey(key, value))
	}
}

// JSONHasKey checks if a JSONB column contains a specific top-level key.
func JSONHasKey(column string, key string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(datatypes.JSONQuery(column).HasKey(key))
	}
}

// ArrayContains generates a Postgres array intersection/contains query.
// E.g., db.Scopes(db.ArrayContains("tags", "go"))
func ArrayContains(column string, value any) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(fmt.Sprintf("%s @> ?", column), value)
	}
}

// TextSearch generates a raw Postgres to_tsvector / to_tsquery full-text search.
func TextSearch(column string, query string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// Basic english text search dict
		return db.Where(fmt.Sprintf("to_tsvector('english', %s) @@ plainto_tsquery('english', ?)", column), query)
	}
}
