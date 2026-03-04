package validate

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Validator wraps go-playground/validator with custom configurations.
type Validator struct {
	v *validator.Validate
}

// New creates a new Validator with custom rules registered.
func New(db *pgxpool.Pool) *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())

	if db != nil {
		v.RegisterValidation("exists", existsRule(db))
		v.RegisterValidation("unique", uniqueRule(db))
	}
	v.RegisterValidation("after_date", afterDateRule)

	return &Validator{v: v}
}

// ValidateStruct validates a struct and returns structured ValidationErrors.
func (v *Validator) ValidateStruct(obj any) error {
	err := v.v.Struct(obj)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return fmt.Errorf("validation: %w", err)
	}

	ve := NewValidationErrors()
	for _, fe := range validationErrors {
		field := toSnakeCase(fe.Field())
		msg := formatMessage(fe)
		ve.Add(field, msg)
	}

	return ve
}

func formatMessage(fe validator.FieldError) string {
	field := toSnakeCase(fe.Field())
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, fe.Param())
	case "exists":
		return fmt.Sprintf("selected %s does not exist", field)
	case "unique":
		return fmt.Sprintf("%s has already been taken", field)
	default:
		return fmt.Sprintf("%s failed on '%s' validation", field, fe.Tag())
	}
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
