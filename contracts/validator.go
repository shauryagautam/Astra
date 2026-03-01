package contracts

// ValidationError represents a single validation failure.
type ValidationError struct {
	// Field is the field name that failed validation.
	Field string `json:"field"`

	// Rule is the validation rule that failed.
	Rule string `json:"rule"`

	// Message is the human-readable error message.
	Message string `json:"message"`
}

// ValidationResult holds the outcome of a validation operation.
type ValidationResult struct {
	// Valid is true if all validations passed.
	Valid bool `json:"valid"`

	// Errors contains all validation failures.
	Errors []ValidationError `json:"errors,omitempty"`
}

// HasErrors returns true if there are validation errors.
func (v *ValidationResult) HasErrors() bool {
	return len(v.Errors) > 0
}

// ErrorMap returns errors grouped by field name.
func (v *ValidationResult) ErrorMap() map[string][]string {
	result := make(map[string][]string)
	for _, e := range v.Errors {
		result[e.Field] = append(result[e.Field], e.Message)
	}
	return result
}

// FirstError returns the first error message or empty string.
func (v *ValidationResult) FirstError() string {
	if len(v.Errors) > 0 {
		return v.Errors[0].Message
	}
	return ""
}

// FieldRule defines a single validation rule.
type FieldRule struct {
	// Name is the rule identifier (e.g., "required", "minLength").
	Name string

	// Params holds rule-specific parameters.
	Params map[string]any

	// Message is an optional custom error message.
	Message string
}

// FieldSchema defines validation rules for a single field.
type FieldSchema struct {
	// Field is the field name.
	Field string

	// Rules is the ordered list of rules to apply.
	Rules []FieldRule

	// Type is the expected type ("string", "number", "boolean", "array", "object").
	Type string
}

// ValidatorContract defines the validation engine interface.
// Mirrors Astra's @astra/validator.
type ValidatorContract interface {
	// Validate validates the given data against the schema.
	Validate(data map[string]any, schema []FieldSchema) *ValidationResult
}
