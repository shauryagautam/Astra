package test_util

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"


	"github.com/shauryagautam/Astra/pkg/engine/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
)


// TestResponse wraps httptest.ResponseRecorder with fluent assertions.
type TestResponse struct {
	Recorder *httptest.ResponseRecorder
	t        *testing.T
}

// AssertStatus asserts the response status code.
func (r *TestResponse) AssertStatus(code int) *TestResponse {
	assert.Equal(r.t, code, r.Recorder.Code)
	return r
}

// ExpectStatus is an alias for AssertStatus.
func (r *TestResponse) ExpectStatus(code int) *TestResponse {
	return r.AssertStatus(code)
}

// AssertJSON asserts the response body contains a specific JSON value at the given key.
func (r *TestResponse) AssertJSON(key string, expected any) *TestResponse {
	var body map[string]any
	err := json.Unmarshal(r.Recorder.Body.Bytes(), &body)
	require.NoError(r.t, err, "Response body is not valid JSON")

	val, ok := body[key]
	assert.True(r.t, ok, "Key %q not found in JSON response", key)
	assert.Equal(r.t, expected, val)
	return r
}

// ExpectJSON is an alias for AssertJSON.
func (r *TestResponse) ExpectJSON(key string, expected any) *TestResponse {
	return r.AssertJSON(key, expected)
}

// AssertJSONCount asserts the length of a JSON array at the given key.
func (r *TestResponse) AssertJSONCount(key string, count int) *TestResponse {
	var body map[string]any
	err := json.Unmarshal(r.Recorder.Body.Bytes(), &body)
	require.NoError(r.t, err, "Response body is not valid JSON")

	val, ok := body[key]
	assert.True(r.t, ok, "Key %q not found in JSON response", key)

	arr, ok := val.([]any)
	assert.True(r.t, ok, "Key %q is not a JSON array", key)
	assert.Equal(r.t, count, len(arr))
	return r
}

// AssertHeader asserts the response contains a specific header value.
func (r *TestResponse) AssertHeader(key, expected string) *TestResponse {
	val := r.Recorder.Header().Get(key)
	assert.Equal(r.t, expected, val, "Header %q does not match", key)
	return r
}

// AssertBodyContains checks if the response body contains the given string.
func (r *TestResponse) AssertBodyContains(s string) *TestResponse {
	assert.Contains(r.t, r.Recorder.Body.String(), s)
	return r
}

// AssertContract verifies that the response body matches the structural contract
// defined by the provided model (Go struct).
func (r *TestResponse) AssertContract(model any) *TestResponse {
	var body map[string]any
	err := json.Unmarshal(r.Recorder.Body.Bytes(), &body)
	require.NoError(r.t, err, "Response body is not valid JSON")

	// Use reflection to verify keys
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		r.t.Fatalf("AssertContract: expected a struct or pointer to struct, got %v", t.Kind())
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		
		name := jsonTag
		if name == "" {
			name = field.Name
		} else if idx := strings.Index(name, ","); idx != -1 {
			name = name[:idx]
		}

		_, ok := body[name]
		assert.True(r.t, ok, "Contract Violation: Key %q missing from response (Type: %v)", name, t.Name())
	}

	return r
}

// Debug prints the response body for debugging purposes.

func (r *TestResponse) Debug() *TestResponse {
	fmt.Printf("\n--- Debug Response ---\nStatus: %d\nBody: %s\n----------------------\n", r.Recorder.Code, r.Recorder.Body.String())
	return r
}
