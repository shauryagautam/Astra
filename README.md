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
