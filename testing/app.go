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
	App    *core.App
	Router *astrahttp.Router
	t      *testing.T
}

// NewTestApp creates a new application for testing.
func NewTestApp(t *testing.T, setup func(app *core.App, router *astrahttp.Router)) *TestApp {
	app, err := core.New()
	require.NoError(t, err)

	router := astrahttp.NewRouter(app)
	setup(app, router)

	return &TestApp{
		App:    app,
		Router: router,
		t:      t,
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

func (a *TestApp) do(req *http.Request) *TestResponse {
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, req)
	return &TestResponse{
		Recorder: w,
		t:        a.t,
	}
}
