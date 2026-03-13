package ws

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestHub(t *testing.T) {
	h := NewHub(nil, "")
	go h.Run()
	defer h.Stop(context.Background())

	t.Run("Register and Unregister", func(t *testing.T) {
		conn := &Connection{
			send:  make(chan []byte, 1),
			rooms: make(map[string]bool),
			hub:   h,
		}

		h.register <- conn
		time.Sleep(10 * time.Millisecond) // wait for hub loop

		h.mu.RLock()
		assert.True(t, h.connections[conn])
		h.mu.RUnlock()

		h.unregister <- conn
		time.Sleep(10 * time.Millisecond)

		h.mu.RLock()
		assert.False(t, h.connections[conn])
		h.mu.RUnlock()
	})

	t.Run("Room Management", func(t *testing.T) {
		conn := &Connection{
			send:  make(chan []byte, 1),
			rooms: make(map[string]bool),
			hub:   h,
		}
		h.register <- conn

		h.JoinRoom(conn, "test-room")
		h.mu.RLock()
		assert.True(t, h.rooms["test-room"][conn])
		h.mu.RUnlock()

		// Broadcast to room
		err := h.BroadcastToRoom("test-room", "greet", "hello")
		assert.NoError(t, err)

		select {
		case msg := <-conn.send:
			var data map[string]any
			json.Unmarshal(msg, &data)
			assert.Equal(t, "greet", data["event"])
			assert.Equal(t, "hello", data["data"])
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for message")
		}

		h.LeaveRoom(conn, "test-room")
		h.mu.RLock()
		assert.Nil(t, h.rooms["test-room"])
		h.mu.RUnlock()
	})
}

func TestSSEServer(t *testing.T) {
	s := NewSSEServer()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	done := make(chan bool)
	go func() {
		s.Handler(w, req, func(events chan<- SSEEvent) {
			events <- SSEEvent{Event: "test", Data: "hello", ID: "1"}
			cancel() // Stop the handler
		})
		done <- true
	}()

	select {
	case <-done:
		assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
		body := w.Body.String()
		assert.Contains(t, body, "event: test")
		assert.Contains(t, body, "data: hello")
		assert.Contains(t, body, "id: 1")
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for SSE")
	}
}

func TestUpgrader(t *testing.T) {
	wsCfg := config.WSConfig{
		AllowedOrigins: []string{"http://trusted.com"},
	}

	t.Run("Dev Mode Allowed", func(t *testing.T) {
		u := NewUpgrader(nil, wsCfg, true)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://evil.com")
		assert.True(t, u.upgrader.CheckOrigin(req))
	})

	t.Run("Prod Mode Restricted", func(t *testing.T) {
		u := NewUpgrader(nil, wsCfg, false)

		req1 := httptest.NewRequest("GET", "/", nil)
		req1.Header.Set("Origin", "http://trusted.com")
		assert.True(t, u.upgrader.CheckOrigin(req1))

		req2 := httptest.NewRequest("GET", "/", nil)
		req2.Header.Set("Origin", "http://evil.com")
		assert.False(t, u.upgrader.CheckOrigin(req2))
	})
}
