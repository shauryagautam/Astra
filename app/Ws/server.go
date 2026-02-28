package ws

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/shaurya/adonis/contracts"
)

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	mu         sync.RWMutex
	clients    map[string]*Client
	rooms      map[string]map[string]*Client
	onConnect  func(client contracts.WsClientContract)
	register   chan *Client
	unregister chan *Client
	upgrader   websocket.Upgrader
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		rooms:      make(map[string]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// Start starts the hub's registration loop.
func (h *Hub) Start() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.id] = client
			h.mu.Unlock()
			if h.onConnect != nil {
				h.onConnect(client)
			}
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.id]; ok {
				delete(h.clients, client.id)
				close(client.sendCh)
				// Remove from rooms
				for room := range client.rooms {
					h.leaveRoom(room, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Handle upgrades the HTTP connection to a WebSocket.
func (h *Hub) Handle(w http.ResponseWriter, r *http.Request) error {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	// For simplicity, we'll use the remote address as a string ID.
	id := fmt.Sprintf("conn_%s", conn.RemoteAddr().String())
	client := NewClient(id, h, conn)

	h.register <- client

	go client.readLoop()
	go client.writeLoop()

	return nil
}

func (h *Hub) Broadcast(event string, payload any) {
	msg := contracts.WsEvent{Event: event, Payload: payload}
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.sendCh <- msg:
		default:
			// Client channel full or closed
		}
	}
}

func (h *Hub) BroadcastToRoom(room string, event string, payload any) {
	msg := contracts.WsEvent{Event: event, Payload: payload}
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.rooms[room]; ok {
		for _, client := range clients {
			select {
			case client.sendCh <- msg:
			default:
				// Client channel full or closed
			}
		}
	}
}

func (h *Hub) OnConnect(fn func(client contracts.WsClientContract)) {
	h.onConnect = fn
}

func (h *Hub) joinRoom(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[string]*Client)
	}
	h.rooms[room][client.id] = client
}

func (h *Hub) leaveRoom(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.rooms[room]; ok {
		delete(clients, client.id)
		if len(clients) == 0 {
			delete(h.rooms, room)
		}
	}
}

// Ensure Hub implements WsServerContract.
var _ contracts.WsServerContract = (*Hub)(nil)
