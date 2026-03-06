# Astra Framework

Astra is a production-grade Go full-stack framework inspired by AdonisJS and Ruby on Rails.

## Features

- **HTTP Router:** Fast, flexible routing with middleware support based on `go-chi/chi`.
- **Database:** Fully integrated with `pgx/v5`, optimized for Postgres and NeonDB.
- **Cache & Queue:** Redis-backed caching and background jobs.
- **Authentication:** Built-in JWT authentication.
- **WebSockets:** Real-time communication via rooms and broadcasting.
- **Storage:** Local and S3-compatible file storage.
- **Mail:** SMTP and Resend.com support.
- **CLI:** Powerful scaffolding and code generation tool.
- **TypeScript Codegen:** Automatically generate a type-safe API client for React/Expo.
- **Unique Jobs:** Built-in deduplication for background tasks with `DispatchUnique`.
- **Typed IoC:** Type-safe service retrieval using Go 1.18+ generics (`Get[T]`).
- **Performance:** Bytedance Sonic integrated for high-performance JSON across the entire framework.
- **SSR Engine:** Enhanced with Flash messages, Asset helpers (`asset_path`, `asset_tag`), and CSRF template integration.
- **Advanced Redis:** Exclusive support for Redis Streams (Queue), Sentinel, and Cluster configurations via `UniversalClient`.

## Quickstart

### 1. Install CLI
```bash
go install github.com/astraframework/astra/cli@latest
```

### 2. Create a new app
```bash
astra new myapp
cd myapp
```

### 3. Start development server
```bash
astra dev
```

### Example
See `_examples/todo-api` for a complete working API with Auth, Websockets, Jobs, and more.

## Architecture

Astra follows a service-oriented architecture where everything is injected via the service container.

```go
func main() {
    app, _ := core.New()
    
    app.OnStart(func(ctx context.Context) error {
        router := http.NewRouter(app)
        // your routes...
        return nil
    })
    
    app.Start()
}
```

## Testing

Astra provides a first-class testing experience with mockable drivers:

```go
func TestUserCreation(t *testing.T) {
    app := testing.NewTestApp(t, setup)
    mailer := testing.NewFakeMailer()
    app.Register("mailer", mailer)
    
    resp := app.POST("/users", `{"email": "test@example.com"}`)
    resp.AssertStatus(201)
    
    mailer.AssertSent(t, "test@example.com")
}
```

## Security

Ready for production with:
- Configurable WebSocket origin whitelist.
- Secure HTTP headers (CSP, HSTS, etc.) applied by default.
- Request body size limits.
- Fail-safe validation rules.
- JWT secret length enforcement.

