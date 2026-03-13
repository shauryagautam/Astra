package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContext_JSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{
		Writer:  rec,
		Request: req,
	}

	data := map[string]string{"message": "hello"}
	err := c.JSON(data, http.StatusCreated)
	require.NoError(t, err)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"message":"hello"}`, rec.Body.String())
}

func TestContext_HTML(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{
		Writer:  rec,
		Request: req,
	}

	html := "<h1>hello</h1>"
	err := c.HTML(html)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, html, rec.Body.String())
}

func TestContext_Bind(t *testing.T) {
	body := []byte(`{"name":"Astra"}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	c := &Context{
		Writer:  rec,
		Request: req,
	}

	type Payload struct {
		Name string `json:"name"`
	}
	var p Payload
	err := c.Bind(&p)
	require.NoError(t, err)
	require.Equal(t, "Astra", p.Name)
}

func TestContext_NoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &Context{
		Writer: rec,
	}

	err := c.NoContent()
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
}
