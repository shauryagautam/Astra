package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// DBExecutor is the minimal interface the validator needs to run DB-backed rules.
// It is satisfied by *sql.DB, *orm.DB, or any custom adapter.
//
// Migration from the old API:
//
//	// Before:
//	validate.New(db)
//
//	// After:
//	validate.New(validate.WithDB(dbAdapter))
//	// or simply:
//	validate.New()  // no DB rules (exists/unique will be skipped)
type DBExecutor interface {
	// QueryRow executes a query expected to return at most one row.
	// The query must accept a single text argument and return a single integer
	// (the count). This is sufficient for EXISTS and UNIQUE checks.
	QueryRow(ctx context.Context, sql string, args ...any) DBRow
}

// DBRow is the minimal interface for scanning a single value from a query.
type DBRow interface {
	Scan(dest ...any) error
}

// MessageFormatter is a function that formats a field error into a human-readable string.
// It can optionally take a locale for i18n support.
type MessageFormatter func(fe validator.FieldError, locale ...string) string

// ValidatorOption configures the Validator.
type ValidatorOption func(*Validator)

// WithDB attaches a DB executor for database-backed validation rules
// (exists, unique). Without this, those rules silently pass.
func WithDB(db DBExecutor) ValidatorOption {
	return func(v *Validator) { v.db = db }
}

// WithMessageFormatter overrides the default English error message formatter.
// Useful for i18n integration.
func WithMessageFormatter(formatter MessageFormatter) ValidatorOption {
	return func(v *Validator) { v.msgFmt = formatter }
}

// WithCustomRule registers a custom validation rule.
func WithCustomRule(tag string, fn validator.Func) ValidatorOption {
	return func(v *Validator) {
		_ = v.v.RegisterValidation(tag, fn)
	}
}

// Validator wraps go-playground/validator with custom configurations.
type Validator struct {
	v      *validator.Validate
	db     DBExecutor
	msgFmt MessageFormatter
}

// New creates a new Validator. Pass options to configure it:
//
//	v := validate.New(validate.WithDB(dbAdapter), validate.WithMessageFormatter(i18nFn))
func New(opts ...ValidatorOption) *Validator {
	v := &Validator{
		v:      validator.New(validator.WithRequiredStructEnabled()),
		msgFmt: formatMessage, // default english formatter
	}

	// Apply options first (so db is set before registering DB rules).
	for _, o := range opts {
		if o != nil {
			o(v)
		}
	}

	// Register built-in rules.
	_ = v.v.RegisterValidation("after_date", afterDateRule)

	// Register DB rules only if a DB was provided.
	if v.db != nil {
		_ = v.v.RegisterValidation("exists", existsRule(v.db))
		_ = v.v.RegisterValidation("unique", uniqueRule(v.db))
	}

	return v
}

// ValidateStruct validates a struct and returns structured ValidationErrors.
// It respects the locale if provided in the context (via i18n middleware).
func (v *Validator) ValidateStruct(obj any, locale ...string) error {
	err := v.v.Struct(obj)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return fmt.Errorf("validate: %w", err)
	}

	lang := ""
	if len(locale) > 0 {
		lang = locale[0]
	}

	ve := NewValidationErrors()
	for _, fe := range validationErrors {
		field := toSnakeCase(fe.Field())
		msg := v.msgFmt(fe, lang)
		ve.Add(field, msg)
	}

	return ve
}

func formatMessage(fe validator.FieldError, locale ...string) string {
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
	case "after_date":
		return fmt.Sprintf("%s must be after %s", field, fe.Param())
	default:
		return fmt.Sprintf("%s failed on '%s' validation", field, fe.Tag())
	}
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := s[i-1]
			if prev < 'A' || prev > 'Z' {
				result.WriteByte('_')
			}
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
