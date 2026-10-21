package database

import "errors"

var (
	// ErrMissingDatabase indicates that a fixture was asked to persist without
	// an explicitly supplied database.
	ErrMissingDatabase = errors.New("orm: fixture requires an explicit database")
	// ErrInvalidFixtureCount indicates that a negative fixture count was given.
	ErrInvalidFixtureCount = errors.New("orm: fixture count must not be negative")
)
