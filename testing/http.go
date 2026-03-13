package testing

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/astraframework/astra/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Debug prints the response body for debugging purposes.
func (r *TestResponse) Debug() *TestResponse {
	fmt.Printf("\n--- Debug Response ---\nStatus: %d\nBody: %s\n----------------------\n", r.Recorder.Code, r.Recorder.Body.String())
	return r
}
