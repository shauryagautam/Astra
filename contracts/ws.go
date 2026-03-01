package contracts

import "net/http"

// WsEvent represents a websocket event.
type WsEvent struct {
	Event   string `json:"event"`
	Payload any    `json:"payload"`
	Room    string `json:"room,omitempty"`
}

// WsClientContract defines a single websocket connection.
type WsClientContract interface {
	// Send sends an event to the client.
	Send(event string, payload any) error

	// Join joins a room.
	Join(room string)

	// Leave leaves a room.
	Leave(room string)

	// Close closes the connection.
	Close() error

	// Id returns the client ID.
	Id() string
}

// WsServerContract defines the websocket manager.
// Mirrors Astra's WebSocket module.
type WsServerContract interface {
	// Handle handles the websocket upgrade request.
	Handle(w http.ResponseWriter, r *http.Request) error

	// Broadcast sends an event to all clients.
	Broadcast(event string, payload any)

	// BroadcastToRoom sends an event to all clients in a room.
	BroadcastToRoom(room string, event string, payload any)

	// OnConnect registers a callback for new connections.
	OnConnect(fn func(client WsClientContract))
}
