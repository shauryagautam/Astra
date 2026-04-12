package test_util

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
	"github.com/shauryagautam/Astra/pkg/engine/json"
)

// TestApp is a test wrapper around the Astra app.
type TestApp struct {
	App     *engine.App
	Router  *astrahttp.Router
	t       *testing.T
	headers map[string]string
}

// NewTestApp creates a new application for testing.
func NewTestApp(t *testing.T, setup func(app *engine.App, router *astrahttp.Router)) *TestApp {
	cfg := &config.AstraConfig{}
	env := &config.Config{}
	logger := slog.Default()
	
	app := engine.New(cfg, env, logger)
	
	// Create a decoupled router for testing
	router := astrahttp.NewRouter(cfg, logger)
	
	if setup != nil {
		setup(app, router)
	}

	return &TestApp{
		App:     app,
		Router:  router,
		t:       t,
		headers: make(map[string]string),
	}
}

func (a *TestApp) WithHeader(key, value string) *TestApp {
	newHeaders := make(map[string]string)
	for k, v := range a.headers {
		newHeaders[k] = v
	}
	newHeaders[key] = value

	return &TestApp{
		App:     a.App,
		Router:  a.Router,
		t:       a.t,
		headers: newHeaders,
	}
}

func (a *TestApp) GET(path string) *TestResponse {
	req := httptest.NewRequest("GET", path, nil)
	return a.do(req)
}

// POST is a generic helper for making POST requests.
func (a *TestApp) POST(path string, body any) *TestResponse {
	return POST(a, path, body)
}

func POST[T any](a *TestApp, path string, body T) *TestResponse {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	return a.do(req)
}

// PUT is a generic helper for making PUT requests.
func (a *TestApp) PUT(path string, body any) *TestResponse {
	return PUT(a, path, body)
}

func PUT[T any](a *TestApp, path string, body T) *TestResponse {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
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
