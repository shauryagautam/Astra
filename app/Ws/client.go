package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/shaurya/astra/contracts"
)

// Client implements the WsClientContract.
type Client struct {
	id     string
	hub    *Hub
	conn   *websocket.Conn
	mu     sync.Mutex
	rooms  map[string]bool
	sendCh chan contracts.WsEvent
}

// NewClient creates a new WsClient.
func NewClient(id string, hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		id:     id,
		hub:    hub,
		conn:   conn,
		rooms:  make(map[string]bool),
		sendCh: make(chan contracts.WsEvent, 256),
	}
}

func (c *Client) Id() string {
	return c.id
}

func (c *Client) Send(event string, payload any) error {
	c.sendCh <- contracts.WsEvent{Event: event, Payload: payload}
	return nil
}

func (c *Client) Join(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rooms[room] = true
	c.hub.joinRoom(room, c)
}

func (c *Client) Leave(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rooms, room)
	c.hub.leaveRoom(room, c)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// readLoop reads messages from the websocket connection.
func (c *Client) readLoop() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var event contracts.WsEvent
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		// Handle client-initiated events (like joining rooms)
		switch event.Event {
		case "join":
			if room, ok := event.Payload.(string); ok {
				c.Join(room)
			}
		case "leave":
			if room, ok := event.Payload.(string); ok {
				c.Leave(room)
			}
		}

		// Emit the event to the hub's internal event system if needed
		// For now, we'll just log or broadcast it.
		// Hub handles broadcasting logic.
	}
}

// writeLoop writes messages to the websocket connection.
func (c *Client) writeLoop() {
	defer func() {
		c.conn.Close()
	}()

	for event := range c.sendCh {
		message, err := json.Marshal(event)
		if err != nil {
			continue
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
