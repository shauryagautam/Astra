package testing

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponse wraps httptest.ResponseRecorder with assertions.
type TestResponse struct {
	Recorder *httptest.ResponseRecorder
	t        *testing.T
}

// AssertStatus asserts the response status code.
func (r *TestResponse) AssertStatus(code int) {
	assert.Equal(r.t, code, r.Recorder.Code)
}

// AssertJSON asserts the response body contains a specific JSON value at the given key.
func (r *TestResponse) AssertJSON(key string, expected any) {
	var body map[string]any
	err := json.Unmarshal(r.Recorder.Body.Bytes(), &body)
	require.NoError(r.t, err, "Response body is not valid JSON")

	val, ok := body[key]
	assert.True(r.t, ok, "Key %s not found in JSON response", key)
	assert.Equal(r.t, expected, val)
}
