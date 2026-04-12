package http

import (
	stdhttp "net/http"

	"github.com/gorilla/websocket"
	"github.com/shauryagautam/Astra/pkg/engine" // Fine, http already imports engine
	"github.com/shauryagautam/Astra/pkg/realtime"
)

// WebSocketHandler handles WebSocket connections.
type WebSocketHandler struct {
	upgrader *websocket.Upgrader
	manager  *realtime.RoomManager
	app      *engine.App
}

func NewWebSocketHandler(manager *realtime.RoomManager, app *engine.App) *WebSocketHandler {
	return &WebSocketHandler{
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *stdhttp.Request) bool { return true },
		},
		manager: manager,
		app:     app,
	}
}

func (h *WebSocketHandler) Connect(c *Context) error {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return err
	}
	_ = conn // Handle connection
	return nil
}
