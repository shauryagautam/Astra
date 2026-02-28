// Package validator provides a schema-based validation engine for Adonis Go.
// Mirrors AdonisJS's @adonisjs/validator with a fluent schema builder API.
//
// Usage:
//
//	v := validator.New()
//	result := v.Validate(ctx.Request().All(), []contracts.FieldSchema{
//	    validator.String("name").Required().MinLength(2).MaxLength(100).Schema(),
//	    validator.String("email").Required().Email().Schema(),
//	    validator.Number("age").Required().Min(18).Max(120).Schema(),
//	    validator.Boolean("active").Schema(),
//	})
//
//	if result.HasErrors() {
//	    return ctx.Response().Status(422).Json(result)
//	}
package validator

import (
	"fmt"
	"math"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/shaurya/adonis/contracts"
)

// Validator is the core validation engine.
type Validator struct{}

// New creates a new Validator.
func New() *Validator {
	return &Validator{}
}

// Validate validates data against the given schema.
func (v *Validator) Validate(data map[string]any, schema []contracts.FieldSchema) *contracts.ValidationResult {
	result := &contracts.ValidationResult{Valid: true}

	for _, field := range schema {
		value, exists := data[field.Field]

		for _, rule := range field.Rules {
			if err := applyRule(field.Field, value, exists, rule, field.Type); err != nil {
				result.Valid = false
				msg := err.Error()
				if rule.Message != "" {
					msg = rule.Message
				}
				result.Errors = append(result.Errors, contracts.ValidationError{
					Field:   field.Field,
					Rule:    rule.Name,
					Message: msg,
				})
				// Stop on first failure for this field (fail-fast per field)
				break
			}
		}
	}

	return result
}

// Ensure Validator implements ValidatorContract.
var _ contracts.ValidatorContract = (*Validator)(nil)

// applyRule dispatches to the appropriate rule checker.
func applyRule(field string, value any, exists bool, rule contracts.FieldRule, fieldType string) error {
	switch rule.Name {
	case "required":
		return ruleRequired(field, value, exists)
	case "optional":
		return nil // always passes, stops chain if not present
	case "minLength":
		return ruleMinLength(field, value, exists, rule.Params)
	case "maxLength":
		return ruleMaxLength(field, value, exists, rule.Params)
	case "min":
		return ruleMin(field, value, exists, rule.Params)
	case "max":
		return ruleMax(field, value, exists, rule.Params)
	case "email":
		return ruleEmail(field, value, exists)
	case "url":
		return ruleURL(field, value, exists)
	case "uuid":
		return ruleUUID(field, value, exists)
	case "alpha":
		return ruleAlpha(field, value, exists)
	case "alphaNum":
		return ruleAlphaNum(field, value, exists)
	case "numeric":
		return ruleNumeric(field, value, exists)
	case "in":
		return ruleIn(field, value, exists, rule.Params)
	case "notIn":
		return ruleNotIn(field, value, exists, rule.Params)
	case "regex":
		return ruleRegex(field, value, exists, rule.Params)
	case "confirmed":
		return ruleConfirmed(field, value, exists, rule.Params)
	case "boolean":
		return ruleBoolean(field, value, exists)
	case "ip":
		return ruleIP(field, value, exists)
	case "json":
		return ruleJSON(field, value, exists)
	case "date":
		return ruleDate(field, value, exists)
	case "array":
		return ruleArray(field, value, exists)
	case "enum":
		return ruleIn(field, value, exists, rule.Params) // alias for in
	default:
		return nil
	}
}

// ══════════════════════════════════════════════════════════════════════
// Rule Implementations
// ══════════════════════════════════════════════════════════════════════

func ruleRequired(field string, value any, exists bool) error {
	if !exists || value == nil {
		return fmt.Errorf("%s is required", field)
	}
	if s, ok := value.(string); ok && strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func ruleMinLength(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return nil
	}
	min := getIntParam(params, "min", 0)
	if len([]rune(s)) < min {
		return fmt.Errorf("%s must be at least %d characters", field, min)
	}
	return nil
}

func ruleMaxLength(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return nil
	}
	max := getIntParam(params, "max", math.MaxInt32)
	if len([]rune(s)) > max {
		return fmt.Errorf("%s must be at most %d characters", field, max)
	}
	return nil
}

func ruleMin(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	num := toFloat64(value)
	min := getFloat64Param(params, "min", 0)
	if num < min {
		return fmt.Errorf("%s must be at least %v", field, min)
	}
	return nil
}

func ruleMax(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	num := toFloat64(value)
	max := getFloat64Param(params, "max", math.MaxFloat64)
	if num > max {
		return fmt.Errorf("%s must be at most %v", field, max)
	}
	return nil
}

func ruleEmail(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a valid email address", field)
	}
	_, err := mail.ParseAddress(s)
	if err != nil {
		return fmt.Errorf("%s must be a valid email address", field)
	}
	return nil
}

func ruleURL(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a valid URL", field)
	}
	u, err := url.ParseRequestURI(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must be a valid URL", field)
	}
	return nil
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ruleUUID(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok || !uuidRegex.MatchString(s) {
		return fmt.Errorf("%s must be a valid UUID", field)
	}
	return nil
}

func ruleAlpha(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must contain only letters", field)
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return fmt.Errorf("%s must contain only letters", field)
		}
	}
	return nil
}

func ruleAlphaNum(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must contain only letters and numbers", field)
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return fmt.Errorf("%s must contain only letters and numbers", field)
		}
	}
	return nil
}

func ruleNumeric(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return nil
	case string:
		s := value.(string)
		for _, r := range s {
			if !unicode.IsDigit(r) && r != '.' && r != '-' {
				return fmt.Errorf("%s must be numeric", field)
			}
		}
		return nil
	}
	return fmt.Errorf("%s must be numeric", field)
}

func ruleIn(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	allowed, ok := params["values"].([]any)
	if !ok {
		return nil
	}
	s := fmt.Sprintf("%v", value)
	for _, a := range allowed {
		if fmt.Sprintf("%v", a) == s {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %v", field, allowed)
}

func ruleNotIn(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	disallowed, ok := params["values"].([]any)
	if !ok {
		return nil
	}
	s := fmt.Sprintf("%v", value)
	for _, d := range disallowed {
		if fmt.Sprintf("%v", d) == s {
			return fmt.Errorf("%s must not be one of: %v", field, disallowed)
		}
	}
	return nil
}

func ruleRegex(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must match the required pattern", field)
	}
	pattern, _ := params["pattern"].(string)
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%s has an invalid regex pattern", field)
	}
	if !re.MatchString(s) {
		return fmt.Errorf("%s must match the required pattern", field)
	}
	return nil
}

func ruleConfirmed(field string, value any, exists bool, params map[string]any) error {
	if !exists || value == nil {
		return nil
	}
	data, ok := params["data"].(map[string]any)
	if !ok {
		return nil
	}
	confirmField := field + "_confirmation"
	confirmValue, confirmExists := data[confirmField]
	if !confirmExists {
		return fmt.Errorf("%s confirmation is required", field)
	}
	if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", confirmValue) {
		return fmt.Errorf("%s confirmation does not match", field)
	}
	return nil
}

func ruleBoolean(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	switch value.(type) {
	case bool:
		return nil
	case string:
		s := strings.ToLower(value.(string))
		if s == "true" || s == "false" || s == "1" || s == "0" || s == "yes" || s == "no" {
			return nil
		}
	case float64:
		v := value.(float64)
		if v == 0 || v == 1 {
			return nil
		}
	}
	return fmt.Errorf("%s must be a boolean", field)
}

func ruleIP(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok || net.ParseIP(s) == nil {
		return fmt.Errorf("%s must be a valid IP address", field)
	}
	return nil
}

func ruleJSON(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a valid JSON string", field)
	}
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		return nil
	}
	return fmt.Errorf("%s must be a valid JSON string", field)
}

func ruleDate(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a valid date", field)
	}
	// Try common date formats
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"01/02/2006",
	}
	for _, f := range formats {
		if _, err := parseDate(s, f); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%s must be a valid date", field)
}

func ruleArray(field string, value any, exists bool) error {
	if !exists || value == nil {
		return nil
	}
	switch value.(type) {
	case []any:
		return nil
	case []string:
		return nil
	case []int:
		return nil
	case []float64:
		return nil
	}
	return fmt.Errorf("%s must be an array", field)
}

// ══════════════════════════════════════════════════════════════════════
// Schema Builder (Fluent API)
// ══════════════════════════════════════════════════════════════════════

// SchemaBuilder provides a fluent API for building field validation schemas.
type SchemaBuilder struct {
	field string
	typ   string
	rules []contracts.FieldRule
}

// String creates a string field schema builder.
func String(name string) *SchemaBuilder {
	return &SchemaBuilder{field: name, typ: "string"}
}

// Number creates a number field schema builder.
func Number(name string) *SchemaBuilder {
	return &SchemaBuilder{field: name, typ: "number"}
}

// Boolean creates a boolean field schema builder.
func Boolean(name string) *SchemaBuilder {
	return &SchemaBuilder{field: name, typ: "boolean"}
}

// Array creates an array field schema builder.
func Array(name string) *SchemaBuilder {
	return &SchemaBuilder{field: name, typ: "array"}
}

// Object creates an object field schema builder.
func Object(name string) *SchemaBuilder {
	return &SchemaBuilder{field: name, typ: "object"}
}

// Schema finalizes and returns the FieldSchema.
func (s *SchemaBuilder) Schema() contracts.FieldSchema {
	return contracts.FieldSchema{
		Field: s.field,
		Rules: s.rules,
		Type:  s.typ,
	}
}

// Required adds the required rule.
func (s *SchemaBuilder) Required() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "required"})
	return s
}

// Optional marks the field as optional (no error if missing).
func (s *SchemaBuilder) Optional() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "optional"})
	return s
}

// MinLength adds minimum length validation.
func (s *SchemaBuilder) MinLength(min int) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "minLength",
		Params: map[string]any{"min": min},
	})
	return s
}

// MaxLength adds maximum length validation.
func (s *SchemaBuilder) MaxLength(max int) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "maxLength",
		Params: map[string]any{"max": max},
	})
	return s
}

// Min adds minimum value validation.
func (s *SchemaBuilder) Min(min float64) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "min",
		Params: map[string]any{"min": min},
	})
	return s
}

// Max adds maximum value validation.
func (s *SchemaBuilder) Max(max float64) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "max",
		Params: map[string]any{"max": max},
	})
	return s
}

// Email adds email format validation.
func (s *SchemaBuilder) Email() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "email"})
	return s
}

// URL adds URL format validation.
func (s *SchemaBuilder) URL() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "url"})
	return s
}

// UUID adds UUID format validation.
func (s *SchemaBuilder) UUID() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "uuid"})
	return s
}

// Alpha adds alphabetic-only validation.
func (s *SchemaBuilder) Alpha() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "alpha"})
	return s
}

// AlphaNum adds alphanumeric-only validation.
func (s *SchemaBuilder) AlphaNum() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "alphaNum"})
	return s
}

// Numeric adds numeric validation.
func (s *SchemaBuilder) Numeric() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "numeric"})
	return s
}

// In adds inclusion validation.
func (s *SchemaBuilder) In(values ...any) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "in",
		Params: map[string]any{"values": values},
	})
	return s
}

// NotIn adds exclusion validation.
func (s *SchemaBuilder) NotIn(values ...any) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "notIn",
		Params: map[string]any{"values": values},
	})
	return s
}

// Regex adds pattern validation.
func (s *SchemaBuilder) Regex(pattern string) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "regex",
		Params: map[string]any{"pattern": pattern},
	})
	return s
}

// Confirmed adds confirmation field validation (e.g., password_confirmation).
func (s *SchemaBuilder) Confirmed(data map[string]any) *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{
		Name:   "confirmed",
		Params: map[string]any{"data": data},
	})
	return s
}

// IP adds IP address validation.
func (s *SchemaBuilder) IP() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "ip"})
	return s
}

// IsJSON adds JSON format validation.
func (s *SchemaBuilder) IsJSON() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "json"})
	return s
}

// Date adds date format validation.
func (s *SchemaBuilder) Date() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "date"})
	return s
}

// IsArray adds array type validation.
func (s *SchemaBuilder) IsArray() *SchemaBuilder {
	s.rules = append(s.rules, contracts.FieldRule{Name: "array"})
	return s
}

// Enum is an alias for In.
func (s *SchemaBuilder) Enum(values ...any) *SchemaBuilder {
	return s.In(values...)
}

// Message sets a custom error message for the last added rule.
func (s *SchemaBuilder) Message(msg string) *SchemaBuilder {
	if len(s.rules) > 0 {
		s.rules[len(s.rules)-1].Message = msg
	}
	return s
}

// ══════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int8:
		return float64(n)
	case int16:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint:
		return float64(n)
	case uint8:
		return float64(n)
	case uint16:
		return float64(n)
	case uint32:
		return float64(n)
	case uint64:
		return float64(n)
	default:
		return 0
	}
}

func getIntParam(params map[string]any, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case int64:
			return int(n)
		}
	}
	return defaultVal
}

func getFloat64Param(params map[string]any, key string, defaultVal float64) float64 {
	if v, ok := params[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return defaultVal
}

func parseDate(value string, layout string) (time.Time, error) {
	return time.Parse(layout, value)
}
