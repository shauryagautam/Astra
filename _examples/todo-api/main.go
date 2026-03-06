package main

import (
	"context"
	"fmt"
	"log"

	"github.com/astraframework/astra/auth"
	"github.com/astraframework/astra/cache"
	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/db"
	astrahttp "github.com/astraframework/astra/http"
	"github.com/astraframework/astra/http/middleware"
	"github.com/astraframework/astra/mail"
	"github.com/astraframework/astra/queue"
	"github.com/astraframework/astra/ws"
)

func main() {
	app, err := core.New()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	// Register services
	dbService := db.New(app.Config.Database)
	if err := dbService.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start db: %v", err)
	}
	app.Register("db", dbService)
	cacheSvc := cache.New(app.Config.Redis)
	if err := cacheSvc.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start cache: %v", err)
	}
	app.Register("redis", cacheSvc)

	// Run Migrations for Todo
	err = dbService.Orm.AutoMigrate(&Todo{})
	if err != nil {
		log.Fatalf("Failed to migrate: %v", err)
	}

	// App start hook
	app.OnStart(func(ctx context.Context) error {
		// Initialize router
		router := astrahttp.NewRouter(app)

		// Middlewares
		redisSvc := app.MustGet("redis").(*cache.Redis)
		router.Use(middleware.RequestID())
		router.Use(middleware.Logger(app.Logger))
		router.Use(middleware.Recover(app.Logger))
		router.Use(middleware.RateLimiter(redisSvc.Client, 100, 60))

		// WS Hub
		hub := ws.NewHub(redisSvc.Client, "astra_ws_broadcast")
		go hub.Run()
		wsUpgrader := ws.NewUpgrader(hub, app.Config.WS, app.Config.App.Environment == "development")

		// Setup JWT Manager
		jwtManager := auth.NewJWTManager(app.Config.Auth, redisSvc.Client)
		authGuard := &auth.JWTGuard{Manager: jwtManager}

		// Setup Queue
		dispatcher := queue.NewDispatcher(redisSvc.Client, "queue:")

		// Basic health route
		router.Get("/", func(c *astrahttp.Context) error {
			return c.JSON(map[string]string{"message": "Welcome to Astra Todo API"})
		})

		// Auth routes
		authGroup := astrahttp.NewRouter(app)
		authGroup.Post("/register", func(c *astrahttp.Context) error {
			type RegisterReq struct {
				Email    string `json:"email" validate:"required,email"`
				Password string `json:"password" validate:"required,min=6"`
			}
			var req RegisterReq
			if err := c.BindAndValidate(&req); err != nil {
				return c.ValidationError(err)
			}

			// Mock DB save
			userID := "123"

			// Dispatch Welcome Email Job
			dispatcher.Dispatch(c.Ctx(), &WelcomeEmailJob{Email: req.Email}, "welcome_email")

			return c.Created(map[string]string{"user_id": userID})
		})
		authGroup.Post("/login", func(c *astrahttp.Context) error {
			// Mock login
			userID := "123"
			tokenPair, err := jwtManager.IssueTokenPair(c.Ctx(), userID, nil)
			if err != nil {
				return err
			}
			return c.JSON(tokenPair)
		})
		router.Mount("/auth", authGroup.Handler())

		// Protected Todos
		todos := astrahttp.NewRouter(app)
		todos.Use(middleware.Auth(authGuard))
		todos.Get("/", func(c *astrahttp.Context) error {
			// user := c.AuthUser()
			// Fetch from DB or Cache, handle pagination
			return c.JSON(map[string]interface{}{"data": []string{"Todo 1"}})
		})
		todos.Post("/", func(c *astrahttp.Context) error {
			// Create logic, broadcast to WS
			hub.BroadcastToRoom("user_123", "todo_created", map[string]string{"title": "New Todo"})
			return c.Created(map[string]string{"status": "created"})
		})
		router.Mount("/todos", todos.Handler())

		// WebSocket
		router.Get("/ws", func(c *astrahttp.Context) error {
			conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, "123")
			if err != nil {
				return err
			}
			conn.Join("user_123")
			return nil
		})

		// Start HTTP server
		serverAddr := fmt.Sprintf("%s:%d", app.Config.App.Host, app.Config.App.Port)
		app.Logger.Info("HTTP server starting", "addr", serverAddr)
		go func() {
			astrahttp.ListenAndServe(serverAddr, router.Handler())
		}()

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatalf("App failed: %v", err)
	}
}

// WelcomeEmailJob sends a welcome email.
type WelcomeEmailJob struct {
	queue.BaseJob
	Email string `json:"email"`
}

func (j *WelcomeEmailJob) Handle() error {
	fmt.Printf("Sending welcome email to %s\n", j.Email)
	mailer := mail.NewSMTPMailer(config.MailConfig{
		SMTPHost: "localhost", SMTPPort: 1025,
	})
	return mailer.Send(context.Background(), &mail.Message{
		To:      []string{j.Email},
		Subject: "Welcome to Astra",
		Body:    "Thanks for registering!",
	})
}

// Todo represents a sample Todo model using the extended GORM base.
type Todo struct {
	db.Model
	Title       string  `gorm:"type:varchar(255);not null" json:"title"`
	Description string  `gorm:"type:text" json:"description"`
	Completed   bool    `gorm:"default:false" json:"completed"`
	Tags        db.JSON `gorm:"type:jsonb" json:"tags"` // Test JSONB
}
