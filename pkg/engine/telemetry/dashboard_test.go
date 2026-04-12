package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboard_RingBuffer(t *testing.T) {
	dash := NewDashboard(3)

	dash.Track("event", "e1", nil)
	dash.Track("event", "e2", nil)
	dash.Track("event", "e3", nil)

	entries := dash.Entries()
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "e1", entries[0].Message)

	// Overwrite
	dash.Track("event", "e4", nil)
	entries = dash.Entries()
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "e2", entries[0].Message)
	assert.Equal(t, "e4", entries[2].Message)
}

func TestDashboard_TrackLog(t *testing.T) {
	dash := NewDashboard(10)
	dash.TrackLog("INFO", "test log", map[string]any{"foo": "bar"})

	entries := dash.Entries()
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "log", entries[0].Type)
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "test log", entries[0].Message)
}

func TestDashboard_TrackRequest(t *testing.T) {
	dash := NewDashboard(10)
	dash.TrackRequest("GET", "/users", 200, 1500000) // 1.5ms

	entries := dash.Entries()
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "http", entries[0].Type)
	assert.Equal(t, "GET /users", entries[0].Message)

	data := entries[0].Data.(map[string]any)
	assert.Equal(t, "GET", data["method"])
	assert.Equal(t, int64(1), data["ms"]) // 1.5ms rounds down to 1ms
}

func TestDashboard_Clear(t *testing.T) {
	dash := NewDashboard(10)
	dash.Track("event", "e1", nil)
	dash.Clear()

	assert.Equal(t, 0, len(dash.Entries()))
}
