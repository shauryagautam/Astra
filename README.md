<div align="center">

# ⚡ Astra

**The Astra-inspired Go Framework — Elegant, Powerful, Production-Ready**

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)

*Build beautiful API backends in Go with the developer experience you love from Astra*

</div>

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 🏗️ **IoC Container** | Dependency injection with bind, singleton, alias, and fakes for testing |
| 🛣️ **Routing** | Dynamic routes, groups, resources, named routes, middleware |
| 🛡️ **Middleware** | CORS, Logger, Security Headers, Recovery, Auth/Guest/SilentAuth |
| 🗄️ **Lucid ORM** | Generic models, chainable query builder, hooks, relationships, pagination |
| 📦 **Migrations** | Run, rollback, reset, refresh, status tracking |
| ✅ **Validation** | Schema-based validation with 20+ built-in rules and fluent API |
| 🔐 **Authentication** | JWT + Opaque Access Token (OAT) guards with pluggable user providers |
| 🔑 **Hashing** | Argon2id + Bcrypt drivers with auto-detection and rehash checks |
| 🔴 **Redis** | Multi-connection manager, caching, rate limiting, pub/sub, sessions |
| ⚡ **Error Handling** | Centralized exception handler with HttpException and stack traces |
| ⚙️ **Configuration** | `.env` file parser with typed getters (string, int, bool, duration) |
| 🧪 **Testing** | TestApp, TestRequest, TestResponse assertions, mock HttpContext |
| 🖥️ **Ace CLI** | Scaffold controllers, models, migrations, middleware, providers |
| 🏭 **Providers** | Service provider architecture for modular application design |

---

## 📦 Installation

```bash
# Clone the framework
git clone https://github.com/shaurya/astra.git my-app
cd my-app

# Install dependencies
go mod tidy

# Start the development server
go run server.go
```

Your server is now running at `http://localhost:3333` 🚀

---

## 🚀 Quick Start

### 1. Define Routes (`start/routes.go`)

```go
func RegisterRoutes(app contracts.ApplicationContract) {
    Route := app.Use("Route").(*astraHttp.Router)

    Route.Get("/", func(ctx contracts.HttpContextContract) error {
        return ctx.Response().Json(map[string]any{
            "message": "Hello, World!",
        })
    })

    // Grouped routes with prefix and middleware
    Route.Group(func(api contracts.RouterContract) {
        api.Get("/users", usersController.Index)
        api.Post("/users", usersController.Store)
        api.Get("/users/:id", usersController.Show)
    }).Prefix("/api/v1").Middleware("auth")

    // Resource routing (auto-generates CRUD routes)
    Route.Resource("posts", &PostsController{})
}
```

### 2. Validate Input

```go
import "github.com/shaurya/astra/app/Validator"

func CreateUser(ctx contracts.HttpContextContract) error {
    body := ctx.Request().All()

    v := validator.New()
    result := v.Validate(body, []contracts.FieldSchema{
        validator.String("name").Required().MinLength(2).MaxLength(100).Schema(),
        validator.String("email").Required().Email().Schema(),
        validator.Number("age").Required().Min(18).Max(120).Schema(),
        validator.String("role").Required().In("admin", "user", "guest").Schema(),
    })

    if result.HasErrors() {
        return exceptions.UnprocessableEntity("Validation failed", result.Errors)
    }

    // Process valid data...
}
```

### 3. Handle Errors

```go
import "github.com/shaurya/astra/app/Exceptions"

// Return structured errors from handlers
return exceptions.NotFound("User not found")
return exceptions.Unauthorized("Invalid credentials")
return exceptions.UnprocessableEntity("Validation failed", errors)

// Wire the centralized handler to the server
server.SetExceptionHandler(exceptions.NewHandler(debugMode))
```

### 4. Use the ORM

```go
// Find a record
user, err := models.Find[User](db, 1)

// Query with the fluent builder
users, err := models.Query[User](db).
    Where("active = ?", true).
    OrderBy("created_at", "desc").
    Paginate(1, 20)

// Create a record
user := &User{Name: "Alice", Email: "alice@example.com"}
err := models.Create(db, user)
```

---

## 📁 Project Structure

```
├── app/
│   ├── Auth/            # JWT & OAT authentication guards
│   ├── Controllers/     # HTTP controllers
│   ├── Exceptions/      # Centralized error handler
│   ├── Hash/            # Argon2id & Bcrypt hashing
│   ├── Http/            # Router, Server, Context, Middleware
│   ├── Middleware/       # Auth, Guest, SilentAuth middleware
│   ├── Models/          # Lucid ORM (BaseModel, QueryBuilder, Hooks)
│   ├── Redis/           # Redis manager, Cache, RateLimiter, Sessions
│   └── Validator/       # Schema validation engine
├── cmd/astra/          # Ace CLI commands
├── config/              # App, Database, CORS, Auth, Redis, Env config
├── contracts/           # Interface definitions (IoC, HTTP, ORM, Auth, etc.)
├── database/            # Migrations & Seeders
├── examples/api/        # Example REST API
├── providers/           # Service providers (App, Route, Database, Auth, Redis)
├── start/               # Route & middleware registration
├── testing/             # Test utilities
├── server.go            # Application entry point
├── .env.example         # Environment configuration template
└── LICENSE              # MIT License
```

---

## 📡 Advanced Features

### 1. Event System (`contracts/event.go`, `app/Events/dispatcher.go`)
Synchronous and asynchronous event dispatching logic.
```go
Event.On("user:registered", func(data any) error {
    user := data.(*User)
    fmt.Printf("Welcome %s!\n", user.Name)
    return nil
})

// Emit event
Event.Emit("user:registered", user)
```

### 2. Queue / Job System (`contracts/queue.go`, `app/Queue/queue.go`)
Redis-backed background job processing with delayed jobs and worker support.
```go
// Push job to queue
Queue.Push(&SendEmailJob{Email: "user@example.com"})

// Push with delay (in seconds)
Queue.Later(60, &ReminderJob{Id: 123})
```

### 3. File Storage / Drive (`contracts/drive.go`, `app/Drive/drive.go`)
Storage abstraction with support for multiple disks (currently Local).
```go
// Save file
Drive.Put("avatars/user_1.png", contents)

// Save stream
Drive.PutStream("docs/report.pdf", reader)

// Get URL
url := Drive.Url("avatars/user_1.png")
```

### 4. Mail System (`contracts/mail.go`, `app/Mail/smtp.go`)
Fluent API for sending emails with support for background queuing.
```go
Mail.Send(ctx, contracts.MailMessage{
    To: []string{"user@example.com"},
    Subject: "Welcome!",
    HtmlView: "<h1>Hello!</h1>",
})

// Send in background via Queue system
Mail.SendLater(ctx, message)
```

### 5. WebSocket Support (`contracts/ws.go`, `app/Ws/server.go`)
Hub-based WebSocket server with room management and broadcasting.
```go
Ws.OnConnect(func(client contracts.WsClientContract) {
    client.Join("chat:room_1")
    client.Send("welcome", "Hello!")
})

// Broadcast to room
Ws.BroadcastToRoom("chat:room_1", "new_message", msg)
```

---

## 🖥️ CLI Commands

```bash
# Development
go run cmd/astra/main.go serve            # Start server
go run cmd/astra/main.go serve --watch     # Start with hot-reload (Air)

# Scaffolding
go run cmd/astra/main.go make:controller UsersController
go run cmd/astra/main.go make:controller PostsController --resource
go run cmd/astra/main.go make:model User --migration
go run cmd/astra/main.go make:migration create_posts_table
go run cmd/astra/main.go make:middleware RateLimiter
go run cmd/astra/main.go make:provider PaymentProvider

# Database
go run cmd/astra/main.go migration:run
go run cmd/astra/main.go migration:rollback
go run cmd/astra/main.go migration:status
go run cmd/astra/main.go migration:reset
go run cmd/astra/main.go migration:refresh
go run cmd/astra/main.go db:seed
```

---

## ⚙️ Configuration

Copy `.env.example` to `.env` and customize:

```bash
cp .env.example .env
```

Key configuration variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_NAME` | Astra | Application name |
| `APP_ENV` | development | Environment (development/production) |
| `APP_KEY` | — | Secret key for JWT signing |
| `PORT` | 3333 | HTTP server port |
| `DB_HOST` | 127.0.0.1 | PostgreSQL host |
| `DB_DATABASE` | astra_dev | Database name |
| `REDIS_HOST` | 127.0.0.1 | Redis host |
| `HASH_DRIVER` | argon2 | Hash driver (argon2/bcrypt) |

---

## 🧪 Testing

```go
import astratesting "github.com/shaurya/astra/testing"

func TestUsersAPI(t *testing.T) {
    app := astratesting.NewTestApp()
    app.RegisterRoutes(func(router *astraHttp.Router) {
        router.Get("/users", myHandler)
    })

    resp := app.Get("/users").Expect(t)
    resp.AssertOk()
    resp.AssertJson("data", expectedData)
    resp.AssertJsonHasKey("data")

    // POST with JSON body
    resp = app.Post("/users").
        WithJSON(map[string]string{"name": "Alice"}).
        WithAuth("jwt-token").
        Expect(t)
    resp.AssertCreated()
}
```

Run all tests:

```bash
go test ./... -v
```

---

## ✅ Validation Rules

| Rule | Usage | Description |
|------|-------|-------------|
| `Required()` | `String("name").Required()` | Field must be present and non-empty |
| `MinLength(n)` | `String("name").MinLength(3)` | Minimum string length |
| `MaxLength(n)` | `String("name").MaxLength(100)` | Maximum string length |
| `Min(n)` | `Number("age").Min(18)` | Minimum numeric value |
| `Max(n)` | `Number("age").Max(120)` | Maximum numeric value |
| `Email()` | `String("email").Email()` | Valid email format |
| `URL()` | `String("website").URL()` | Valid URL format |
| `UUID()` | `String("id").UUID()` | Valid UUID format |
| `Alpha()` | `String("name").Alpha()` | Letters only |
| `AlphaNum()` | `String("code").AlphaNum()` | Letters and numbers only |
| `Numeric()` | `String("code").Numeric()` | Numeric characters only |
| `In(...)` | `String("role").In("admin", "user")` | Value must be in list |
| `NotIn(...)` | `String("role").NotIn("banned")` | Value must not be in list |
| `Regex(pattern)` | `String("code").Regex("^[A-Z]+$")` | Match regex pattern |
| `IP()` | `String("addr").IP()` | Valid IP address |
| `Date()` | `String("dob").Date()` | Valid date format |
| `Boolean()` | `Boolean("active").Required()` | Boolean value |
| `Enum(...)` | `String("status").Enum("on", "off")` | Same as `In()` |
| `Message(msg)` | `.Required().Message("Custom msg")` | Custom error message |

---

## 🔐 Authentication

```go
// JWT Authentication
guard := authManager.Use("jwt")
token, err := guard.Attempt(ctx, map[string]any{
    "email":    "user@example.com",
    "password": "secret",
})

// Protect routes with auth middleware
Route.Get("/dashboard", handler).Middleware("auth")
```

---

## 📄 License

[MIT License](LICENSE) © 2026 Shaurya
