package main

import (
	"context"

	"github.com/shaurya/astra/contracts"
)

func registerAdvancedRoutes(app contracts.ApplicationContract) {
	Route := app.Use("Route").(contracts.RouterContract)
	Drive := app.Use("Drive").(contracts.DriveContract)
	Mail := app.Use("Mail").(contracts.MailerContract)
	Ws := app.Use("Ws").(contracts.WsServerContract)

	// --- 1. WebSocket Route ---
	Route.Get("/ws", func(ctx contracts.HttpContextContract) error {
		return Ws.Handle(ctx.Response().Raw(), ctx.Request().Raw())
	})

	// --- 2. File Storage (Drive) ---
	Route.Post("/upload", func(ctx contracts.HttpContextContract) error {
		// Mock upload
		path := "uploads/test.txt"
		content := []byte("Hello from Astra Drive!")

		if err := Drive.Put(path, content); err != nil {
			return err
		}

		return ctx.Response().Json(map[string]any{
			"message": "File uploaded",
			"url":     Drive.Url(path),
		})
	})

	// --- 3. Mail & Queue ---
	Route.Post("/send-email", func(ctx contracts.HttpContextContract) error {
		msg := contracts.MailMessage{
			To:       []string{"user@example.com"},
			Subject:  "Welcome to Astra!",
			HtmlView: "<h1>Advanced Features</h1><p>This was sent via the Queue system.</p>",
		}

		// Send in background
		if err := Mail.SendLater(context.Background(), msg); err != nil {
			return err
		}

		return ctx.Response().Json(map[string]any{"message": "Email queued"})
	})

	// --- 4. Events ---
	Route.Get("/trigger-event", func(ctx contracts.HttpContextContract) error {
		Event := app.Use("Event").(contracts.EventDispatcherContract)
		Event.Emit("user:reached_endpoint", map[string]any{"path": "/trigger-event"})

		return ctx.Response().Json(map[string]any{"message": "Event emitted"})
	})
}
