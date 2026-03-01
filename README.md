<div align="center">
  # ⚡ Astra

  **The Elegant, Powerful, and Production-Ready Web Framework for Go.**


  [![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go)](https://go.dev)
  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)](LICENSE)
  [![DB: PostgreSQL](https://img.shields.io/badge/Database-PostgreSQL-336791?style=for-the-badge&logo=postgresql)](https://postgresql.org/)
  [![Cache: Redis](https://img.shields.io/badge/Cache-Redis-DC382D?style=for-the-badge&logo=redis)](https://redis.io/)


  <p>
    <em>Build elite API backends in Go with the expressive developer experience of Laravel and AdonisJS, powered by statically typed performance.</em>
  </p>
</div>

---

## 🌟 Why Astra?

Astra is not just another routing library; it is a **batteries-included, opinionated ecosystem** designed to solve the complexities of enterprise web development in Go. It embraces pattern-driven architecture, moving away from sparse micro-frameworks into a structured, highly productive environment.

*   **🔋 Batteries Included**: IoC Container, ORM, Auth guards, Router, flexible Schema Validator, Job Queues, and multi-node Redis clusters out-of-the-box.
*   **🏭 Pattern-Driven**: Predictable directory structures, Service Providers, and Middlewares built symmetrically to proven MVC standards.
*   **🛡️ Production Hardened**: Neon Serverless connection pooling, Atomic Lua Rate Limiting, and impenetrable CSP/HSTS security headers enabled by default.
*   **🏎️ Developer Experience (DX)**: Zero-configuration JSON logging (`slog`), the Ace scaffolding CLI, and fluent query builders allow you to focus on business logic unconditionally.

---

## 📦 Requirements & Installation

Astra requires **Go 1.22+**.

To start using Astra in your project, initialize your Go module and fetch the framework:

```bash
# 1. Initialize your project
mkdir my-app && cd my-app
go mod init my-app

# 2. Install the Astra Framework
go get -u github.com/shaurya/astra

# 3. Import and configure your server inside your main.go!
```

---

## 🏗️ The Architectural Tour

### 1. Advanced Routing Engine
Astra’s router is highly expressive, supporting nesting, middleware application, parameter bindings, and instant JSON responses.

```go
func RegisterRoutes(app contracts.ApplicationContract) {
    Route := app.Use("Route").(*astraHttp.Router)

    // A simple endpoint
    Route.Get("/ping", func(ctx contracts.HttpContextContract) error {
        return ctx.Response().Json(map[string]any{"status": "alive"})
    })

    // Powerful Controller routing with Middleware
    Route.Group(func(api contracts.RouterContract) {
        api.Get("/profile", UsersController.Profile)
        api.Post("/settings", UsersController.UpdateSettings)
    }).Prefix("/api/v1").Middleware("auth") // The Auth Guard secures this group
    
    // Auto-maps typical RESTful methods (Index, Show, Store, Update, Destroy) 
    Route.Resource("articles", &ArticlesController{})
}
```

### 2. Validation Schema Builder
Never let bad data touch your controllers. The Astra validator acts as an impenetrable shield, enforcing strict schema compliance with fluent chaining.

```go
func (c *UsersController) Store(ctx contracts.HttpContextContract) error {
    body := ctx.Request().All()

    v := validator.New()
    result := v.Validate(body, []contracts.FieldSchema{
        validator.String("username").Required().MinLength(3).AlphaNum().Schema(),
        validator.String("email").Required().Email().Schema(),
        validator.Number("age").Min(18).Max(99).Schema(),
        validator.String("role").In("admin", "editor", "subscriber").Schema(),
    })

    if result.HasErrors() {
        // Automatically returns a 422 Unprocessable Entity with error mapped fields
        return exceptions.UnprocessableEntity("Validation failed", result.Errors)
    }
    
    // Resume execution...
}
```

### 3. The Lucid ORM
Astra ships with a deeply integrated ORM abstraction mapped onto GORM, but supercharged with connection pooling optimized seamlessly for serverless databases (Neon) and enterprise limits.

```go
// Fetch user with Primary Key
user, _ := models.Find[User](db, 1)

// Fluent Builder syntax
activeAdmins, _ := models.Query[User](db).
    Where("role = ?", "admin").
    Where("status = ?", "active").
    OrderBy("created_at", "desc").
    Paginate(1, 20) // Automatically handles offset and limit
    
// Persisting records
newUser := &User{Email: "hello@astra.dev"}
models.Create(db, newUser)
```

### 4. Enterprise Services (Redis & Queues)
Scaling a monolith? Astra has your back. Redis is natively tied to universally clustered connections.

```go
// 🔴 Redis: Atomic increments and caching
cache := app.Use("Redis").(*redis.RedisManager).Connection("local")
cache.Set(ctx, "last_login", time.Now().Unix(), time.Hour)

// 📬 Job Queues: Push background tasks off the main thread
Queue := app.Use("Queue").(contracts.QueueContract)
Queue.Push(&ProcessImageJob{ImageID: 1045})
Queue.Later(120, &SendWelcomeEmailJob{UserID: 1}) // Executes exactly 120s later
```

---

## 🖥️ The Ace CLI Commands

Astra ships with a built-in scaffolding CLI allowing you to build foundational logic without repetitive typing.

```bash
# Create a new HTTP Controller natively
go run cmd/astra/main.go make:controller AuthController

# Create a generic model with its associated database migration schema
go run cmd/astra/main.go make:model Payment --migration

# Push modifications to PostgreSQL
go run cmd/astra/main.go migration:run

# Refresh the schema database completely for testing
go run cmd/astra/main.go migration:refresh
```

---

## 🛡️ Uncompromising Security

Astra takes the burden of securing your API off your shoulders. 

*   **Atomic Rate Limiting**: Redis integrations use server-side `Lua scripts` guaranteeing atomic IP sliding-window throttling. Auto-scaling Kubernetes pods will never cause a race-condition rate-limit bypass.
*   **Security Headers Middleware**: Enforced default sets of `Strict-Transport-Security (HSTS)`, `X-Content-Type-Options: nosniff`, and extremely restrictive `Content-Security-Policy (CSP)` instructions mitigate XSS/code-injection natively.
*   **Opaque Access Tokens (OAT)**: Fully stateless Hash drivers (`Argon2id // Bcrypt`) secure database-mapped API authentication effortlessly.

---

## 📚 Official Documentation

Explore the official documentation hub built specifically with Astro Starlight to read deep architectural guides on how to implement the IoC container, handle graceful websocket shutdowns, and manage multi-database setups.

Navigate to `/docs` locally to spin up the site, or view our interactive API references!

---

<div align="center">
  <b>Architected with ❤️ by Shaurya and the Open Source Community.</b> <br/>
  <i>Released under the MIT License.</i>
</div>
