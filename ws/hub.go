package ws

import (
	"context"
	"log"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"
)

// Hub manages active WebSocket connections and rooms.
type Hub struct {
	// Registered connections
	connections map[*Connection]bool

	// Rooms map room name to a map of connections
	rooms map[string]map[*Connection]bool

	// Inbound messages from connections
	broadcast chan []byte

	// Register requests from connections
	register chan *Connection

	// Unregister requests from connections
	unregister chan *Connection

	redis redis.UniversalClient
	rChan string

	stop chan struct{}
	mu   sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub(redis redis.UniversalClient, rChan string) *Hub {
	return &Hub{
		broadcast:   make(chan []byte),
		register:    make(chan *Connection),
		unregister:  make(chan *Connection),
		connections: make(map[*Connection]bool),
		rooms:       make(map[string]map[*Connection]bool),
		redis:       redis,
		rChan:       rChan,
		stop:        make(chan struct{}),
	}
}

// Run starts the hub loop and optionally the Redis subscription.
func (h *Hub) Run() {
	if h.redis != nil {
		go h.listenRedis()
	}

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.connections[conn] = true
			h.mu.Unlock()
		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				for room := range conn.rooms {
					if _, ok := h.rooms[room]; ok {
						delete(h.rooms[room], conn)
						if len(h.rooms[room]) == 0 {
							delete(h.rooms, room)
						}
					}
				}
				close(conn.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(h.connections, conn)
				}
			}
			h.mu.RUnlock()
		case <-h.stop:
			h.mu.Lock()
			for conn := range h.connections {
				close(conn.send)
				delete(h.connections, conn)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Stop signals the hub to shut down.
func (h *Hub) Stop(ctx context.Context) error {
	close(h.stop)
	return nil
}

func (h *Hub) listenRedis() {
	pubsub := h.redis.Subscribe(context.Background(), h.rChan)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var payload struct {
			Room  string `json:"room"`
			Event string `json:"event"`
			Data  any    `json:"data"`
		}
		if err := sonic.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			log.Printf("[Astra WS] Invalid Redis message: %v", err)
			continue
		}
		h.broadcastToRoomLocal(payload.Room, payload.Event, payload.Data)
	}
}

// BroadcastToRoom sends a message to all connections in a specific room across all nodes.
func (h *Hub) BroadcastToRoom(room string, event string, data any) error {
	if h.redis != nil {
		payload, _ := sonic.Marshal(map[string]any{
			"room":  room,
			"event": event,
			"data":  data,
		})
		return h.redis.Publish(context.Background(), h.rChan, payload).Err()
	}
	return h.broadcastToRoomLocal(room, event, data)
}

func (h *Hub) broadcastToRoomLocal(room string, event string, data any) error {
	msg := map[string]any{
		"event": event,
		"data":  data,
	}
	bytes, err := sonic.Marshal(msg)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if connections, ok := h.rooms[room]; ok {
		for conn := range connections {
			select {
			case conn.send <- bytes:
			default:
				// handled by unregister
			}
		}
	}
	return nil
}

// JoinRoom adds a connection to a room.
func (h *Hub) JoinRoom(conn *Connection, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*Connection]bool)
	}
	h.rooms[room][conn] = true
	conn.rooms[room] = true
}

// LeaveRoom removes a connection from a room.
func (h *Hub) LeaveRoom(conn *Connection, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[room]; ok {
		delete(h.rooms[room], conn)
		if len(h.rooms[room]) == 0 {
			delete(h.rooms, room)
		}
	}
	delete(conn.rooms, room)
}
