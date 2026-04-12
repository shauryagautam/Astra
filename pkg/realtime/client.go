package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)


// Message represents a WebSocket message
type Message struct {
	Type      string                 `json:"type"`
	From      string                 `json:"from,omitempty"`
	To        string                 `json:"to,omitempty"`
	Room      string                 `json:"room,omitempty"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

// Client represents a connected WebSocket client
type Client struct {
	ID        string
	UserID    string
	Conn      *websocket.Conn
	Send      chan Message
	Manager   *RoomManager
	LastSeen  time.Time
}

// NewClient creates a new client
func NewClient(conn *websocket.Conn, userID string, manager *RoomManager) *Client {
	return &Client{
		ID:       userID + "-" + time.Now().Format("150405"),
		UserID:   userID,
		Conn:     conn,
		Send:     make(chan Message, 256),
		Manager:  manager,
		LastSeen: time.Now(),
	}
}

// ReadPump pumps messages from the websocket connection to the manager.
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.Manager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.LastSeen = time.Now()
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("WebSocket read error", "error", err)
				}
				return
			}

			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				slog.Warn("Failed to unmarshal WebSocket message", "error", err)
				continue
			}

			msg.From = c.UserID
			msg.Timestamp = time.Now()

			if msg.Room != "" {
				c.Manager.Broadcast <- msg
			} else if msg.To != "" {
				c.Manager.SendToUser(msg.To, msg)
			}
		}
	}
}

// WritePump pumps messages from the hub to the websocket connection.
func (c *Client) WritePump(ctx context.Context) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			
			data, _ := json.Marshal(message)
			w.Write(data)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
