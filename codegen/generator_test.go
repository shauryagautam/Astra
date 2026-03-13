package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGoTypeToTS(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "string"},
		{"int", "number"},
		{"bool", "boolean"},
		{"*string", "string | null"},
		{"[]string", "string[]"},
		{"map[string]string", "Record<string, string>"},
		{"time.Time", "string"},
		{"models.User", "User"},
		{"unknown_type", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseGoTypeToTS(tt.goType))
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_profile", "UserProfile"},
		{"get_users", "GetUsers"},
		{"api_v1_auth", "ApiV1Auth"},
		{"index", "Index"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, toPascalCase(tt.input))
		})
	}
}
