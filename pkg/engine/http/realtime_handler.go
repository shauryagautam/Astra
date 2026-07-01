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

	userID := "anonymous"
	if claims := c.AuthUser(); claims != nil {
		userID = claims.UserID
	} else if queryUserID := c.Query("user_id"); queryUserID != "" {
		userID = queryUserID
	}

	client := realtime.NewClient(conn, userID, h.manager)
	h.manager.Register <- client

	// Start writer pump in background
	go client.WritePump(c.Request.Context())
	
	// Start reader pump in current goroutine to block
	client.ReadPump(c.Request.Context())
	return nil
}
