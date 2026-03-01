package main

import (
	"fmt"

	"github.com/shaurya/astra/contracts"
)

func registerAdvancedEvents(app contracts.ApplicationContract) {
	Event := app.Use("Event").(contracts.EventDispatcherContract)
	Ws := app.Use("Ws").(contracts.WsServerContract)

	// 1. Log event
	Event.On("user:reached_endpoint", func(data any) error {
		payload := data.(map[string]any)
		fmt.Printf("[event:log] User reached: %s\n", payload["path"])
		return nil
	})

	// 2. Notify via WebSocket
	Event.On("user:reached_endpoint", func(data any) error {
		payload := data.(map[string]any)
		Ws.Broadcast("server_log", map[string]any{
			"message": fmt.Sprintf("Someone visited %s", payload["path"]),
			"time":    "now",
		})
		return nil
	})

	// 3. One-time setup event
	Event.Once("app:ready", func(data any) error {
		fmt.Println("[event:init] Application is fully ready and advanced features are active.")
		return nil
	})

	// Trigger the ready event
	Event.Emit("app:ready", nil)
}
