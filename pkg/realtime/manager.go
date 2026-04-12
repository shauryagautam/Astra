package realtime

import (
	"sync"
	"time"
)

// Room represents a collection of clients
type Room struct {
	ID          string
	Name        string
	Private     bool
	MaxSize     int
	Clients     map[string]*Client
	Metadata    map[string]interface{}
	Created     time.Time
	mu          sync.RWMutex
}

// RoomManager manages WebSocket clients and rooms
type RoomManager struct {
	Rooms      map[string]*Room
	Clients    map[string]*Client // All connected clients: userID -> Client
	Broadcast  chan Message
	Register   chan *Client
	Unregister chan *Client
	mu         sync.RWMutex
}

// NewRoomManager creates a new room manager
func NewRoomManager() *RoomManager {
	manager := &RoomManager{
		Rooms:      make(map[string]*Room),
		Clients:    make(map[string]*Client),
		Broadcast:  make(chan Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
	go manager.Run()
	return manager
}

// Run starts the room manager loop
func (m *RoomManager) Run() {
	for {
		select {
		case client := <-m.Register:
			m.mu.Lock()
			m.Clients[client.UserID] = client
			m.mu.Unlock()
		case client := <-m.Unregister:
			m.mu.Lock()
			if _, ok := m.Clients[client.UserID]; ok {
				delete(m.Clients, client.UserID)
				close(client.Send)
			}
			// Remove from all rooms
			for _, room := range m.Rooms {
				room.mu.Lock()
				delete(room.Clients, client.UserID)
				room.mu.Unlock()
			}
			m.mu.Unlock()
		case message := <-m.Broadcast:
			if message.Room != "" {
				m.mu.RLock()
				room, ok := m.Rooms[message.Room]
				m.mu.RUnlock()
				if ok {
					room.mu.RLock()
					for _, client := range room.Clients {
						select {
						case client.Send <- message:
						default:
							close(client.Send)
							delete(room.Clients, client.UserID)
						}
					}
					room.mu.RUnlock()
				}
			} else {
				// Global broadcast
				m.mu.RLock()
				for _, client := range m.Clients {
					select {
					case client.Send <- message:
					default:
						// Already closed or full
					}
				}
				m.mu.RUnlock()
			}
		}
	}
}

// CreateRoom creates a new room
func (m *RoomManager) CreateRoom(id, name string, private bool, maxSize int) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	room := &Room{
		ID:       id,
		Name:     name,
		Private:  private,
		MaxSize:  maxSize,
		Clients:  make(map[string]*Client),
		Created:  time.Now(),
		Metadata: make(map[string]interface{}),
	}
	m.Rooms[id] = room
	return room
}

// GetRoom returns a room by ID
func (m *RoomManager) GetRoom(id string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	room, ok := m.Rooms[id]
	return room, ok
}

// SendToUser sends a message to a specific user
func (m *RoomManager) SendToUser(userID string, message Message) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if client, ok := m.Clients[userID]; ok {
		select {
		case client.Send <- message:
		default:
		}
	}
}

// BroadcastToRoom sends a message to all users in a room
func (m *RoomManager) BroadcastToRoom(roomID string, message Message) {
	message.Room = roomID
	m.Broadcast <- message
}

// GetStats returns statistics about the manager
func (m *RoomManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	rooms := make([]map[string]interface{}, 0, len(m.Rooms))
	for id, room := range m.Rooms {
		room.mu.RLock()
		rooms = append(rooms, map[string]interface{}{
			"id":           id,
			"name":         room.Name,
			"client_count": len(room.Clients),
		})
		room.mu.RUnlock()
	}
	
	return map[string]interface{}{
		"client_count": len(m.Clients),
		"room_count":   len(m.Rooms),
		"rooms":        rooms,
	}
}
