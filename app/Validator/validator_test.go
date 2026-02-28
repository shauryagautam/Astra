package validator

import (
	"testing"

	"github.com/shaurya/adonis/contracts"
)

func TestRequiredRule(t *testing.T) {
	v := New()

	// Missing field
	result := v.Validate(map[string]any{}, []contracts.FieldSchema{
		String("name").Required().Schema(),
	})
	if result.Valid {
		t.Fatal("should fail when required field is missing")
	}
	if result.Errors[0].Rule != "required" {
		t.Fatalf("expected rule 'required', got '%s'", result.Errors[0].Rule)
	}

	// Empty string
	result = v.Validate(map[string]any{"name": ""}, []contracts.FieldSchema{
		String("name").Required().Schema(),
	})
	if result.Valid {
		t.Fatal("should fail when required field is empty string")
	}

	// Present field
	result = v.Validate(map[string]any{"name": "John"}, []contracts.FieldSchema{
		String("name").Required().Schema(),
	})
	if !result.Valid {
		t.Fatalf("should pass when required field is present, errors: %v", result.Errors)
	}
}

func TestMinMaxLength(t *testing.T) {
	v := New()

	// Too short
	result := v.Validate(map[string]any{"name": "ab"}, []contracts.FieldSchema{
		String("name").MinLength(3).Schema(),
	})
	if result.Valid {
		t.Fatal("should fail when string is too short")
	}

	// Too long
	result = v.Validate(map[string]any{"name": "abcdef"}, []contracts.FieldSchema{
		String("name").MaxLength(5).Schema(),
	})
	if result.Valid {
		t.Fatal("should fail when string is too long")
	}

	// Just right
	result = v.Validate(map[string]any{"name": "abcd"}, []contracts.FieldSchema{
		String("name").MinLength(3).MaxLength(5).Schema(),
	})
	if !result.Valid {
		t.Fatalf("should pass, errors: %v", result.Errors)
	}
}

func TestMinMaxNumber(t *testing.T) {
	v := New()

	// Below minimum
	result := v.Validate(map[string]any{"age": float64(10)}, []contracts.FieldSchema{
		Number("age").Min(18).Schema(),
	})
	if result.Valid {
		t.Fatal("should fail when number is below minimum")
	}

	// Above maximum
	result = v.Validate(map[string]any{"age": float64(200)}, []contracts.FieldSchema{
		Number("age").Max(120).Schema(),
	})
	if result.Valid {
		t.Fatal("should fail when number is above maximum")
	}

	// In range
	result = v.Validate(map[string]any{"age": float64(25)}, []contracts.FieldSchema{
		Number("age").Min(18).Max(120).Schema(),
	})
	if !result.Valid {
		t.Fatalf("should pass, errors: %v", result.Errors)
	}
}

func TestEmailRule(t *testing.T) {
	v := New()

	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"test.user+tag@domain.co", true},
		{"invalid", false},
		{"@domain.com", false},
		{"user@", false},
	}

	for _, tt := range tests {
		result := v.Validate(map[string]any{"email": tt.email}, []contracts.FieldSchema{
			String("email").Email().Schema(),
		})
		if result.Valid != tt.valid {
			t.Fatalf("email '%s': expected valid=%v, got valid=%v", tt.email, tt.valid, result.Valid)
		}
	}
}

func TestURLRule(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{"url": "https://example.com/path"}, []contracts.FieldSchema{
		String("url").URL().Schema(),
	})
	if !result.Valid {
		t.Fatal("valid URL should pass")
	}

	result = v.Validate(map[string]any{"url": "not-a-url"}, []contracts.FieldSchema{
		String("url").URL().Schema(),
	})
	if result.Valid {
		t.Fatal("invalid URL should fail")
	}
}

func TestUUIDRule(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000"}, []contracts.FieldSchema{
		String("id").UUID().Schema(),
	})
	if !result.Valid {
		t.Fatal("valid UUID should pass")
	}

	result = v.Validate(map[string]any{"id": "not-a-uuid"}, []contracts.FieldSchema{
		String("id").UUID().Schema(),
	})
	if result.Valid {
		t.Fatal("invalid UUID should fail")
	}
}

func TestInRule(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{"role": "admin"}, []contracts.FieldSchema{
		String("role").In("admin", "user", "guest").Schema(),
	})
	if !result.Valid {
		t.Fatal("value in allowed list should pass")
	}

	result = v.Validate(map[string]any{"role": "superuser"}, []contracts.FieldSchema{
		String("role").In("admin", "user", "guest").Schema(),
	})
	if result.Valid {
		t.Fatal("value not in allowed list should fail")
	}
}

func TestAlphaRule(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{"name": "JohnDoe"}, []contracts.FieldSchema{
		String("name").Alpha().Schema(),
	})
	if !result.Valid {
		t.Fatal("alpha string should pass")
	}

	result = v.Validate(map[string]any{"name": "John123"}, []contracts.FieldSchema{
		String("name").Alpha().Schema(),
	})
	if result.Valid {
		t.Fatal("string with numbers should fail alpha rule")
	}
}

func TestRegexRule(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{"code": "ABC-123"}, []contracts.FieldSchema{
		String("code").Regex(`^[A-Z]{3}-\d{3}$`).Schema(),
	})
	if !result.Valid {
		t.Fatal("matching regex should pass")
	}

	result = v.Validate(map[string]any{"code": "abc-123"}, []contracts.FieldSchema{
		String("code").Regex(`^[A-Z]{3}-\d{3}$`).Schema(),
	})
	if result.Valid {
		t.Fatal("non-matching regex should fail")
	}
}

func TestBooleanRule(t *testing.T) {
	v := New()

	boolValues := []any{true, false, "true", "false", "1", "0", float64(1), float64(0)}
	for _, val := range boolValues {
		result := v.Validate(map[string]any{"active": val}, []contracts.FieldSchema{
			Boolean("active").Required().Schema(),
		})
		if !result.Valid {
			t.Fatalf("boolean value %v should pass, errors: %v", val, result.Errors)
		}
	}

	result := v.Validate(map[string]any{"active": "maybe"}, []contracts.FieldSchema{
		{Field: "active", Rules: []contracts.FieldRule{{Name: "boolean"}}},
	})
	if result.Valid {
		t.Fatal("'maybe' should fail boolean rule")
	}
}

func TestIPRule(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{"ip": "192.168.1.1"}, []contracts.FieldSchema{
		String("ip").IP().Schema(),
	})
	if !result.Valid {
		t.Fatal("valid IPv4 should pass")
	}

	result = v.Validate(map[string]any{"ip": "::1"}, []contracts.FieldSchema{
		String("ip").IP().Schema(),
	})
	if !result.Valid {
		t.Fatal("valid IPv6 should pass")
	}

	result = v.Validate(map[string]any{"ip": "999.999.999.999"}, []contracts.FieldSchema{
		String("ip").IP().Schema(),
	})
	if result.Valid {
		t.Fatal("invalid IP should fail")
	}
}

func TestDateRule(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{"birthday": "2024-01-15"}, []contracts.FieldSchema{
		String("birthday").Date().Schema(),
	})
	if !result.Valid {
		t.Fatal("valid date should pass")
	}

	result = v.Validate(map[string]any{"birthday": "not-a-date"}, []contracts.FieldSchema{
		String("birthday").Date().Schema(),
	})
	if result.Valid {
		t.Fatal("invalid date should fail")
	}
}

func TestCustomMessage(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{}, []contracts.FieldSchema{
		String("email").Required().Message("Please provide your email").Schema(),
	})
	if result.Valid {
		t.Fatal("should fail")
	}
	if result.Errors[0].Message != "Please provide your email" {
		t.Fatalf("expected custom message, got '%s'", result.Errors[0].Message)
	}
}

func TestValidationResultHelpers(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{}, []contracts.FieldSchema{
		String("name").Required().Schema(),
		String("email").Required().Schema(),
	})

	if !result.HasErrors() {
		t.Fatal("should have errors")
	}

	errMap := result.ErrorMap()
	if len(errMap["name"]) != 1 {
		t.Fatal("should have 1 error for name")
	}
	if len(errMap["email"]) != 1 {
		t.Fatal("should have 1 error for email")
	}

	first := result.FirstError()
	if first == "" {
		t.Fatal("FirstError should return non-empty string")
	}
}

func TestMultipleRulesChaining(t *testing.T) {
	v := New()

	result := v.Validate(map[string]any{
		"username": "john_doe",
		"email":    "john@example.com",
		"age":      float64(25),
		"role":     "admin",
	}, []contracts.FieldSchema{
		String("username").Required().MinLength(3).MaxLength(20).Schema(),
		String("email").Required().Email().Schema(),
		Number("age").Required().Min(18).Max(120).Schema(),
		String("role").Required().In("admin", "user", "guest").Schema(),
	})

	if !result.Valid {
		t.Fatalf("all valid data should pass, errors: %v", result.Errors)
	}
}

func TestOptionalField(t *testing.T) {
	v := New()

	// Optional field not present — should pass
	result := v.Validate(map[string]any{}, []contracts.FieldSchema{
		String("nickname").Optional().MinLength(3).Schema(),
	})
	// Optional doesn't prevent subsequent rules from running on missing fields,
	// but minLength on nil should be a no-op
	if !result.Valid {
		t.Fatalf("optional missing field should pass, errors: %v", result.Errors)
	}
}
