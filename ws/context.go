package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Connection is a middleman between the websocket connection and the hub.
type Connection struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   string
	rooms    map[string]bool
	handlers map[string]func(json.RawMessage)
	mu       sync.RWMutex
}

// InboundMessage represents a JSON message from the client.
type InboundMessage struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// On registers a handler for a specific event type.
func (c *Connection) On(event string, handler func(json.RawMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[event] = handler
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Connection) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var msg InboundMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		c.mu.RLock()
		handler, ok := c.handlers[msg.Event]
		c.mu.RUnlock()
		if ok {
			go handler(msg.Data)
		} else {
			log.Printf("ws: no handler registered for event: %s", msg.Event)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Emit sends a message to this connection.
func (c *Connection) Emit(event string, data any) error {
	msg := map[string]any{
		"event": event,
		"data":  data,
	}
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.send <- bytes
	return nil
}

// Join joins a room.
func (c *Connection) Join(room string) {
	c.hub.JoinRoom(c, room)
}

// Leave leaves a room.
func (c *Connection) Leave(room string) {
	c.hub.LeaveRoom(c, room)
}
