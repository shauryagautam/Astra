package ws

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

var defaultUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Configurable in production
	},
}

// Upgrader handles upgrading HTTP requests to WebSockets.
type Upgrader struct {
	upgrader websocket.Upgrader
	hub      *Hub
}

// NewUpgrader creates a new WS upgrader.
func NewUpgrader(hub *Hub) *Upgrader {
	return &Upgrader{
		upgrader: defaultUpgrader,
		hub:      hub,
	}
}

// Upgrade upgrades the HTTP request to a WS connection.
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, userID string) (*Connection, error) {
	conn, err := u.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	c := &Connection{
		hub:      u.hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		userID:   userID,
		rooms:    make(map[string]bool),
		handlers: make(map[string]func(json.RawMessage)),
	}

	c.hub.register <- c

	go c.writePump()
	go c.readPump()

	return c, nil
}
