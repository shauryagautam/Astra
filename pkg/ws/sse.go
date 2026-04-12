package ws

import (
	"fmt"
	"net/http"
)

// SSEEvent represents a single server-sent event.
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// SSEServer handles SSE connections.
type SSEServer struct{}

// NewSSEServer creates a new SSE server.
func NewSSEServer() *SSEServer {
	return &SSEServer{}
}

// Handler returns an HTTP handler for SSE.
func (s *SSEServer) Handler(w http.ResponseWriter, r *http.Request, stream func(events chan<- SSEEvent)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	events := make(chan SSEEvent)
	go stream(events)

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if event.ID != "" {
				fmt.Fprintf(w, "id: %s\n", event.ID)
			}
			if event.Event != "" {
				fmt.Fprintf(w, "event: %s\n", event.Event)
			}
			fmt.Fprintf(w, "data: %s\n\n", event.Data)
			flusher.Flush()
		}
	}
}
