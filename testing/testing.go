// Package testing provides test utilities for Astra Go applications.
// Mirrors Astra's testing helpers for building and executing test requests.
//
// Usage:
//
//	func TestHomePage(t *testing.T) {
//	    app := astratesting.NewTestApp()
//	    resp := app.Get("/").Expect(t)
//	    resp.AssertStatus(200)
//	    resp.AssertJson("framework", "Astra Go")
//	}
package astratesting

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	astraApp "github.com/shaurya/astra/app"
	astraHttp "github.com/shaurya/astra/app/Http"
	"github.com/shaurya/astra/contracts"
	"github.com/shaurya/astra/providers"
)

// TestApp provides a testing wrapper around the Astra application.
type TestApp struct {
	App    *astraApp.Application
	Server *astraHttp.Server
	Router *astraHttp.Router
}

// NewTestApp creates a new test application with the core providers bootstrapped.
func NewTestApp() *TestApp {
	application := astraApp.NewApplication(".")

	// Register minimal providers for testing
	application.RegisterProviders([]contracts.ServiceProviderContract{
		providers.NewAppProvider(application),
		providers.NewRouteProvider(application),
	})

	// Boot the application
	if err := application.Boot(); err != nil {
		panic("Failed to boot test app: " + err.Error())
	}

	router := application.Use("Route").(*astraHttp.Router)
	server := application.Use("Server").(*astraHttp.Server)

	return &TestApp{
		App:    application,
		Server: server,
		Router: router,
	}
}

// RegisterRoutes adds routes for testing.
func (t *TestApp) RegisterRoutes(fn func(router *astraHttp.Router)) *TestApp {
	fn(t.Router)
	return t
}

// ══════════════════════════════════════════════════════════════════════
// Test Request Builder
// ══════════════════════════════════════════════════════════════════════

// TestRequest builds an HTTP request for testing.
type TestRequest struct {
	app     *TestApp
	method  string
	path    string
	body    string
	headers map[string]string
}

// Get creates a GET request.
func (a *TestApp) Get(path string) *TestRequest {
	return &TestRequest{app: a, method: "GET", path: path, headers: map[string]string{}}
}

// Post creates a POST request.
func (a *TestApp) Post(path string) *TestRequest {
	return &TestRequest{app: a, method: "POST", path: path, headers: map[string]string{}}
}

// Put creates a PUT request.
func (a *TestApp) Put(path string) *TestRequest {
	return &TestRequest{app: a, method: "PUT", path: path, headers: map[string]string{}}
}

// Delete creates a DELETE request.
func (a *TestApp) Delete(path string) *TestRequest {
	return &TestRequest{app: a, method: "DELETE", path: path, headers: map[string]string{}}
}

// Patch creates a PATCH request.
func (a *TestApp) Patch(path string) *TestRequest {
	return &TestRequest{app: a, method: "PATCH", path: path, headers: map[string]string{}}
}

// WithBody sets the request body string.
func (r *TestRequest) WithBody(body string) *TestRequest {
	r.body = body
	return r
}

// WithJSON sets a JSON body.
func (r *TestRequest) WithJSON(data any) *TestRequest {
	jsonData, _ := json.Marshal(data)
	r.body = string(jsonData)
	r.headers["Content-Type"] = "application/json"
	return r
}

// WithHeader adds a header.
func (r *TestRequest) WithHeader(key, value string) *TestRequest {
	r.headers[key] = value
	return r
}

// WithAuth adds a Bearer token.
func (r *TestRequest) WithAuth(token string) *TestRequest {
	r.headers["Authorization"] = "Bearer " + token
	return r
}

// Expect executes the request and returns a TestResponse.
func (r *TestRequest) Expect(t *testing.T) *TestResponse {
	t.Helper()

	var bodyReader io.Reader
	if r.body != "" {
		bodyReader = strings.NewReader(r.body)
	}
	req := httptest.NewRequest(r.method, r.path, bodyReader)
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	r.app.Server.ServeHTTP(w, req)

	return &TestResponse{
		t:        t,
		Recorder: w,
		Code:     w.Code,
		Body:     w.Body.String(),
	}
}

// ══════════════════════════════════════════════════════════════════════
// Test Response (Assertions)
// ══════════════════════════════════════════════════════════════════════

// TestResponse wraps httptest.ResponseRecorder with assertion helpers.
type TestResponse struct {
	t        *testing.T
	Recorder *httptest.ResponseRecorder
	Code     int
	Body     string
}

// AssertStatus asserts the response has the expected HTTP status code.
func (r *TestResponse) AssertStatus(expected int) *TestResponse {
	r.t.Helper()
	if r.Code != expected {
		r.t.Fatalf("Expected status %d, got %d. Body: %s", expected, r.Code, r.Body)
	}
	return r
}

// AssertOk asserts the response has status 200.
func (r *TestResponse) AssertOk() *TestResponse {
	return r.AssertStatus(http.StatusOK)
}

// AssertCreated asserts the response has status 201.
func (r *TestResponse) AssertCreated() *TestResponse {
	return r.AssertStatus(http.StatusCreated)
}

// AssertNotFound asserts the response has status 404.
func (r *TestResponse) AssertNotFound() *TestResponse {
	return r.AssertStatus(http.StatusNotFound)
}

// AssertUnauthorized asserts the response has status 401.
func (r *TestResponse) AssertUnauthorized() *TestResponse {
	return r.AssertStatus(http.StatusUnauthorized)
}

// AssertContains asserts the response body contains the given string.
func (r *TestResponse) AssertContains(expected string) *TestResponse {
	r.t.Helper()
	if !strings.Contains(r.Body, expected) {
		r.t.Fatalf("Expected body to contain '%s', got: %s", expected, r.Body)
	}
	return r
}

// AssertNotContains asserts the response body does NOT contain the given string.
func (r *TestResponse) AssertNotContains(unexpected string) *TestResponse {
	r.t.Helper()
	if strings.Contains(r.Body, unexpected) {
		r.t.Fatalf("Expected body to NOT contain '%s', got: %s", unexpected, r.Body)
	}
	return r
}

// AssertJson asserts a specific key-value exists in the JSON response.
func (r *TestResponse) AssertJson(key string, expected any) *TestResponse {
	r.t.Helper()

	var data map[string]any
	if err := json.Unmarshal([]byte(r.Body), &data); err != nil {
		r.t.Fatalf("Failed to parse JSON response: %v. Body: %s", err, r.Body)
	}

	actual, exists := data[key]
	if !exists {
		r.t.Fatalf("Expected JSON key '%s' to exist. Response: %s", key, r.Body)
	}

	expectedStr, _ := json.Marshal(expected)
	actualStr, _ := json.Marshal(actual)
	if string(expectedStr) != string(actualStr) {
		r.t.Fatalf("Expected JSON '%s' = %s, got %s", key, expectedStr, actualStr)
	}
	return r
}

// AssertJsonHasKey asserts a key exists in the JSON response.
func (r *TestResponse) AssertJsonHasKey(key string) *TestResponse {
	r.t.Helper()

	var data map[string]any
	if err := json.Unmarshal([]byte(r.Body), &data); err != nil {
		r.t.Fatalf("Failed to parse JSON response: %v", err)
	}
	if _, exists := data[key]; !exists {
		r.t.Fatalf("Expected JSON key '%s' to exist. Response: %s", key, r.Body)
	}
	return r
}

// AssertHeader asserts a response header value.
func (r *TestResponse) AssertHeader(key, expected string) *TestResponse {
	r.t.Helper()
	actual := r.Recorder.Header().Get(key)
	if actual != expected {
		r.t.Fatalf("Expected header '%s' = '%s', got '%s'", key, expected, actual)
	}
	return r
}

// Json parses the response body as JSON into a map.
func (r *TestResponse) Json() map[string]any {
	r.t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(r.Body), &data); err != nil {
		r.t.Fatalf("Failed to parse response JSON: %v", err)
	}
	return data
}

// ══════════════════════════════════════════════════════════════════════
// Test Context
// ══════════════════════════════════════════════════════════════════════

// NewTestContext creates a mock HttpContext for unit-testing handlers directly.
func NewTestContext(method, path string, body ...string) (contracts.HttpContextContract, *httptest.ResponseRecorder) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = strings.NewReader(body[0])
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	w := httptest.NewRecorder()
	ctx := astraHttp.NewHttpContext(w, req)
	return ctx, w
}
