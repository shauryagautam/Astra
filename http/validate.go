package http

import "github.com/astraframework/astra/validate"

// defaultValidator is the package-level validator instance used when no
// App container is available or validator is not registered.
var defaultValidator = validate.New(nil)

// Validate validates a struct using go-playground/validator struct tags.
// Returns a *validate.ValidationErrors if validation fails, nil otherwise.
func Validate(v any) error {
	return defaultValidator.ValidateStruct(v)
}

// ValidationErrors alias for backward compatibility.
type ValidationErrors = validate.ValidationErrors

// NewValidationErrors alias for backward compatibility.
func NewValidationErrors() *ValidationErrors {
	return validate.NewValidationErrors()
}
