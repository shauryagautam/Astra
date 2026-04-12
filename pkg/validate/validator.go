package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
)

// DBExecutor is the minimal interface the validator needs to run DB-backed rules.
// It is satisfied by *sql.DB, *database.DB, or any custom adapter.
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

// Validate validates a struct.
func (v *Validator) Validate(s any) error {
	return v.v.Struct(s)
}

// BindAndValidate decodes the request body and validates the struct.
func (v *Validator) BindAndValidate(r *http.Request, val any) error {
	if err := json.NewDecoder(r.Body).Decode(val); err != nil {
		return err
	}
	return v.Validate(val)
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

// ─── Fluent Validator Set (Logic from validator_new.go) ──────────────

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors map[string]string `json:"errors"`
}

// CustomValidator interface for custom validators (renamed to avoid conflict)
type CustomValidator interface {
	Validate(value any) error
}

// Rule represents a validation rule
type Rule struct {
	Name       string
	Validator  func(any) error
	Message    string
	StopOnFail bool
}

// Field represents a field to be validated
type Field struct {
	Name     string
	Value    any
	Rules    []*Rule
	Required bool
	Optional bool
}

// ValidatorSet represents a collection of validation rules
type ValidatorSet struct {
	fields []*Field
	errors map[string]string
}

// NewValidatorSet creates a new validator set
func NewValidatorSet() *ValidatorSet {
	return &ValidatorSet{
		errors: make(map[string]string),
	}
}

// Field adds a field to be validated
func (vs *ValidatorSet) Field(name string, value any) *FieldBuilder {
	field := &Field{
		Name:  name,
		Value: value,
		Rules: make([]*Rule, 0),
	}
	vs.fields = append(vs.fields, field)
	return &FieldBuilder{field: field}
}

// Validate runs all validations
func (vs *ValidatorSet) Validate() *ValidationResult {
	vs.errors = make(map[string]string)

	for _, field := range vs.fields {
		// Check if field is required but empty
		if field.Required && vs.isEmpty(field.Value) {
			vs.errors[field.Name] = fmt.Sprintf("%s is required", field.Name)
			continue
		}

		// Skip validation if field is optional and empty
		if field.Optional && vs.isEmpty(field.Value) {
			continue
		}

		// Skip validation if field is empty and not required
		if vs.isEmpty(field.Value) && !field.Required {
			continue
		}

		// Run field validations
		for _, rule := range field.Rules {
			if err := rule.Validator(field.Value); err != nil {
				message := rule.Message
				if message == "" {
					message = err.Error()
				}
				vs.errors[field.Name] = message
				if rule.StopOnFail {
					break
				}
			}
		}
	}

	return &ValidationResult{
		Valid:  len(vs.errors) == 0,
		Errors: vs.errors,
	}
}

// isEmpty checks if a value is empty
func (vs *ValidatorSet) isEmpty(value any) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	default:
		return reflect.ValueOf(value).IsZero()
	}
}

// FieldBuilder provides fluent interface for building field validations
type FieldBuilder struct {
	field *Field
}

// Required marks the field as required
func (fb *FieldBuilder) Required() *FieldBuilder {
	fb.field.Required = true
	return fb
}

// Optional marks the field as optional
func (fb *FieldBuilder) Optional() *FieldBuilder {
	fb.field.Optional = true
	return fb
}

// MinLength adds minimum length validation
func (fb *FieldBuilder) MinLength(min int) *FieldBuilder {
	rule := &Rule{
		Name: "min_length",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if utf8.RuneCountInString(str) < min {
				return fmt.Errorf("must be at least %d characters", min)
			}
			return nil
		},
		Message: fmt.Sprintf("must be at least %d characters", min),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// MaxLength adds maximum length validation
func (fb *FieldBuilder) MaxLength(max int) *FieldBuilder {
	rule := &Rule{
		Name: "max_length",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if utf8.RuneCountInString(str) > max {
				return fmt.Errorf("must be at most %d characters", max)
			}
			return nil
		},
		Message: fmt.Sprintf("must be at most %d characters", max),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Email adds email validation
func (fb *FieldBuilder) Email() *FieldBuilder {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	rule := &Rule{
		Name: "email",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if !emailRegex.MatchString(str) {
				return fmt.Errorf("must be a valid email address")
			}
			return nil
		},
		Message: "must be a valid email address",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// URL adds URL validation
func (fb *FieldBuilder) URL() *FieldBuilder {
	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	rule := &Rule{
		Name: "url",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if !urlRegex.MatchString(str) {
				return fmt.Errorf("must be a valid URL")
			}
			return nil
		},
		Message: "must be a valid URL",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Numeric adds numeric validation
func (fb *FieldBuilder) Numeric() *FieldBuilder {
	rule := &Rule{
		Name: "numeric",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if _, err := strconv.ParseFloat(str, 64); err != nil {
				return fmt.Errorf("must be a number")
			}
			return nil
		},
		Message: "must be a number",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Integer adds integer validation
func (fb *FieldBuilder) Integer() *FieldBuilder {
	rule := &Rule{
		Name: "integer",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if _, err := strconv.Atoi(str); err != nil {
				return fmt.Errorf("must be an integer")
			}
			return nil
		},
		Message: "must be an integer",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Min adds minimum value validation
func (fb *FieldBuilder) Min(min float64) *FieldBuilder {
	rule := &Rule{
		Name: "min",
		Validator: func(value any) error {
			var num float64
			var err error

			switch v := value.(type) {
			case string:
				num, err = strconv.ParseFloat(v, 64)
			case float64:
				num = v
			case int:
				num = float64(v)
			default:
				return fmt.Errorf("value must be numeric")
			}

			if err != nil {
				return fmt.Errorf("must be numeric")
			}

			if num < min {
				return fmt.Errorf("must be at least %g", min)
			}
			return nil
		},
		Message: fmt.Sprintf("must be at least %g", min),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Max adds maximum value validation
func (fb *FieldBuilder) Max(max float64) *FieldBuilder {
	rule := &Rule{
		Name: "max",
		Validator: func(value any) error {
			var num float64
			var err error

			switch v := value.(type) {
			case string:
				num, err = strconv.ParseFloat(v, 64)
			case float64:
				num = v
			case int:
				num = float64(v)
			default:
				return fmt.Errorf("value must be numeric")
			}

			if err != nil {
				return fmt.Errorf("must be numeric")
			}

			if num > max {
				return fmt.Errorf("must be at most %g", max)
			}
			return nil
		},
		Message: fmt.Sprintf("must be at most %g", max),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Pattern adds regex pattern validation
func (fb *FieldBuilder) Pattern(pattern string) *FieldBuilder {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		panic(fmt.Sprintf("Invalid regex pattern: %v", err))
	}

	rule := &Rule{
		Name: "pattern",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if !regex.MatchString(str) {
				return fmt.Errorf("must match pattern %s", pattern)
			}
			return nil
		},
		Message: fmt.Sprintf("must match pattern %s", pattern),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Alpha adds alphabetic validation
func (fb *FieldBuilder) Alpha() *FieldBuilder {
	rule := &Rule{
		Name: "alpha",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			for _, r := range str {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
					return fmt.Errorf("must contain only alphabetic characters")
				}
			}
			return nil
		},
		Message: "must contain only alphabetic characters",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// AlphaNumeric adds alphanumeric validation
func (fb *FieldBuilder) AlphaNumeric() *FieldBuilder {
	rule := &Rule{
		Name: "alphanumeric",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			for _, r := range str {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
					return fmt.Errorf("must contain only alphanumeric characters")
				}
			}
			return nil
		},
		Message: "must contain only alphanumeric characters",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// UUID adds UUID validation
func (fb *FieldBuilder) UUID() *FieldBuilder {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	rule := &Rule{
		Name: "uuid",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if !uuidRegex.MatchString(strings.ToLower(str)) {
				return fmt.Errorf("must be a valid UUID")
			}
			return nil
		},
		Message: "must be a valid UUID",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// In adds enum validation
func (fb *FieldBuilder) In(values ...any) *FieldBuilder {
	rule := &Rule{
		Name: "in",
		Validator: func(value any) error {
			for _, v := range values {
				if value == v {
					return nil
				}
			}
			return fmt.Errorf("must be one of: %v", values)
		},
		Message: fmt.Sprintf("must be one of: %v", values),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// NotIn adds negative enum validation
func (fb *FieldBuilder) NotIn(values ...any) *FieldBuilder {
	rule := &Rule{
		Name: "not_in",
		Validator: func(value any) error {
			for _, v := range values {
				if value == v {
					return fmt.Errorf("must not be one of: %v", values)
				}
			}
			return nil
		},
		Message: fmt.Sprintf("must not be one of: %v", values),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Date adds date validation
func (fb *FieldBuilder) Date() *FieldBuilder {
	rule := &Rule{
		Name: "date",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			_, err := time.Parse("2006-01-02", str)
			if err != nil {
				return fmt.Errorf("must be a valid date (YYYY-MM-DD)")
			}
			return nil
		},
		Message: "must be a valid date (YYYY-MM-DD)",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// DateTime adds datetime validation
func (fb *FieldBuilder) DateTime() *FieldBuilder {
	rule := &Rule{
		Name: "datetime",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			_, err := time.Parse(time.RFC3339, str)
			if err != nil {
				return fmt.Errorf("must be a valid datetime (RFC3339)")
			}
			return nil
		},
		Message: "must be a valid datetime (RFC3339)",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Custom adds custom validation
func (fb *FieldBuilder) Custom(validator func(any) error, message string) *FieldBuilder {
	rule := &Rule{
		Name:      "custom",
		Validator: validator,
		Message:   message,
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// OneOf adds validation that field must be one of the specified values
func (fb *FieldBuilder) OneOf(values ...string) *FieldBuilder {
	rule := &Rule{
		Name: "one_of",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			for _, v := range values {
				if str == v {
					return nil
				}
			}
			return fmt.Errorf("must be one of: %s", strings.Join(values, ", "))
		},
		Message: fmt.Sprintf("must be one of: %s", strings.Join(values, ", ")),
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Password adds password validation (at least 8 chars, uppercase, lowercase, number, special)
func (fb *FieldBuilder) Password() *FieldBuilder {
	rule := &Rule{
		Name: "password",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			
			if len(str) < 8 {
				return fmt.Errorf("must be at least 8 characters long")
			}
			
			hasUpper := false
			hasLower := false
			hasNumber := false
			hasSpecial := false
			
			for _, r := range str {
				switch {
				case r >= 'A' && r <= 'Z':
					hasUpper = true
				case r >= 'a' && r <= 'z':
					hasLower = true
				case r >= '0' && r <= '9':
					hasNumber = true
				case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", r):
					hasSpecial = true
				}
			}
			
			if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
				return fmt.Errorf("must contain uppercase, lowercase, number, and special character")
			}
			
			return nil
		},
		Message: "must contain uppercase, lowercase, number, and special character",
		StopOnFail: true,
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Phone adds phone number validation
func (fb *FieldBuilder) Phone() *FieldBuilder {
	phoneRegex := regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
	rule := &Rule{
		Name: "phone",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if !phoneRegex.MatchString(str) {
				return fmt.Errorf("must be a valid phone number")
			}
			return nil
		},
		Message: "must be a valid phone number",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// JSON adds JSON validation
func (fb *FieldBuilder) JSON() *FieldBuilder {
	rule := &Rule{
		Name: "json",
		Validator: func(value any) error {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("value must be a string")
			}
			if !json.Valid([]byte(str)) {
				return fmt.Errorf("must be valid JSON")
			}
			return nil
		},
		Message: "must be valid JSON",
	}
	fb.field.Rules = append(fb.field.Rules, rule)
	return fb
}

// Struct validates a struct using struct tags
func Struct(s any) *ValidationResult {
	vs := NewValidatorSet()
	
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	typ := val.Type()
	
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		
		// Get field name from JSON tag or field name
		name := fieldType.Name
		if tag := fieldType.Tag.Get("json"); tag != "" {
			if parts := strings.Split(tag, ","); len(parts) > 0 && parts[0] != "" {
				name = parts[0]
			}
		}
		
		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}
		
		fb := vs.Field(name, field.Interface())
		
		// Parse validate tag
		if tag := fieldType.Tag.Get("validate"); tag != "" {
			fb.parseValidateTag(tag)
		}
		
		// Check if field is required
		if tag := fieldType.Tag.Get("validate"); strings.Contains(tag, "required") {
			fb.Required()
		}
	}
	
	return vs.Validate()
}

// parseValidateTag parses validation tags
func (fb *FieldBuilder) parseValidateTag(tag string) {
	rules := strings.Split(tag, ",")
	
	for _, rule := range rules {
		parts := strings.Split(rule, "=")
		name := strings.TrimSpace(parts[0])
		
		switch name {
		case "required":
			fb.Required()
		case "optional":
			fb.Optional()
		case "email":
			fb.Email()
		case "url":
			fb.URL()
		case "numeric":
			fb.Numeric()
		case "integer":
			fb.Integer()
		case "alpha":
			fb.Alpha()
		case "alphanumeric":
			fb.AlphaNumeric()
		case "uuid":
			fb.UUID()
		case "password":
			fb.Password()
		case "phone":
			fb.Phone()
		case "json":
			fb.JSON()
		case "date":
			fb.Date()
		case "datetime":
			fb.DateTime()
		case "minlength":
			if len(parts) > 1 {
				if min, err := strconv.Atoi(parts[1]); err == nil {
					fb.MinLength(min)
				}
			}
		case "maxlength":
			if len(parts) > 1 {
				if max, err := strconv.Atoi(parts[1]); err == nil {
					fb.MaxLength(max)
				}
			}
		case "min":
			if len(parts) > 1 {
				if min, err := strconv.ParseFloat(parts[1], 64); err == nil {
					fb.Min(min)
				}
			}
		case "max":
			if len(parts) > 1 {
				if max, err := strconv.ParseFloat(parts[1], 64); err == nil {
					fb.Max(max)
				}
			}
		case "pattern":
			if len(parts) > 1 {
				fb.Pattern(parts[1])
			}
		case "oneof":
			if len(parts) > 1 {
				values := strings.Split(parts[1], "|")
				fb.OneOf(values...)
			}
		}
	}
}
