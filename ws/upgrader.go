package ws

import (
	"github.com/astraframework/astra/json"
	"net/http"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/gorilla/websocket"
)

var defaultUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

// Upgrader handles upgrading HTTP requests to WebSockets.
type Upgrader struct {
	upgrader websocket.Upgrader
	hub      *Hub
}

// NewUpgrader creates a new WS upgrader.
func NewUpgrader(hub *Hub, wsConfig config.WSConfig, isDev bool) *Upgrader {
	upgrader := defaultUpgrader
	upgrader.CheckOrigin = func(r *http.Request) bool {
		if isDev {
			return true
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		for _, allowed := range wsConfig.AllowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	}

	return &Upgrader{
		upgrader: upgrader,
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
