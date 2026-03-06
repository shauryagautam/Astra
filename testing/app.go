package testing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/astraframework/astra/core"
	astrahttp "github.com/astraframework/astra/http"
	"github.com/stretchr/testify/require"
)

// TestApp is a test wrapper around the Astra app.
type TestApp struct {
	App     *core.App
	Router  *astrahttp.Router
	t       *testing.T
	headers map[string]string
}

// NewTestApp creates a new application for testing.
func NewTestApp(t *testing.T, setup func(app *core.App, router *astrahttp.Router)) *TestApp {
	app, err := core.New()
	require.NoError(t, err)

	router := astrahttp.NewRouter(app)
	setup(app, router)

	return &TestApp{
		App:     app,
		Router:  router,
		t:       t,
		headers: make(map[string]string),
	}
}

// WithAuth returns a new TestApp configured with the given JWT bearer token.
func (a *TestApp) WithAuth(token string) *TestApp {
	newHeaders := make(map[string]string)
	for k, v := range a.headers {
		newHeaders[k] = v
	}
	newHeaders["Authorization"] = "Bearer " + token

	return &TestApp{
		App:     a.App,
		Router:  a.Router,
		t:       a.t,
		headers: newHeaders,
	}
}

// GET performs a GET request.
func (a *TestApp) GET(path string) *TestResponse {
	req := httptest.NewRequest("GET", path, nil)
	return a.do(req)
}

// POST performs a POST request.
func (a *TestApp) POST(path string, body string) *TestResponse {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return a.do(req)
}

// PUT interprets as a PUT request body configuration.
func (a *TestApp) PUT(path string, body string) *TestResponse {
	req := httptest.NewRequest("PUT", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return a.do(req)
}

// PATCH interprets as a PATCH request body configuration.
func (a *TestApp) PATCH(path string, body string) *TestResponse {
	req := httptest.NewRequest("PATCH", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return a.do(req)
}

// DELETE interprets as a DELETE request configuration.
func (a *TestApp) DELETE(path string) *TestResponse {
	req := httptest.NewRequest("DELETE", path, nil)
	return a.do(req)
}

func (a *TestApp) do(req *http.Request) *TestResponse {
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, req)
	return &TestResponse{
		Recorder: w,
		t:        a.t,
	}
}
