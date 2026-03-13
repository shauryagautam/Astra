package validate

import (
	"fmt"
)

// ValidationErrors holds structured field validation errors.
type ValidationErrors struct {
	Fields map[string][]string `json:"fields"`
}

// NewValidationErrors creates a new ValidationErrors.
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		Fields: make(map[string][]string),
	}
}

// Add adds an error message for the given field.
func (ve *ValidationErrors) Add(field string, message string) {
	ve.Fields[field] = append(ve.Fields[field], message)
}

// HasErrors returns true if there are any validation errors.
func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Fields) > 0
}

// Error implements the error interface.
func (ve *ValidationErrors) Error() string {
	count := 0
	for _, msgs := range ve.Fields {
		count += len(msgs)
	}
	return fmt.Sprintf("validation failed with %d error(s)", count)
}
