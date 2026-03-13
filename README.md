# Astra Framework

<p align="center">
  <img src="https://raw.githubusercontent.com/shauryagautam/Astra/main/assets/logo.png" width="200" alt="Astra Logo">
</p>

<p align="center">
  <b>A batteries-included, production-grade Go web framework inspired by AdonisJS and Ruby on Rails.</b>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/astraframework/astra"><img src="https://goreportcard.com/badge/github.com/astraframework/astra" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/astraframework/astra"><img src="https://pkg.go.dev/badge/github.com/astraframework/astra.svg" alt="Go Reference"></a>
  <a href="https://github.com/astraframework/astra/actions/workflows/pipeline.yml"><img src="https://github.com/astraframework/astra/actions/workflows/pipeline.yml/badge.svg" alt="CI/CD Pipeline"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</p>

---

Astra provides everything you need to build scalable, secure, and maintainable applications with Go. It prioritizes **Developer Experience (DX)**, **Security by Default**, and **Operational Maturity**.

## 🚀 Key Features

| Category | Capabilities |
| --- | --- |
| **⚡️ Performance** | Multi-threaded Event Emitter, Bytedance Sonic JSON handling, Intelligent Context Pooling |
| **🛡️ Security** | Transparent PII Encryption, PSR-compliant Audit Logging, Native CSRF & XSS protection |
| **📦 Batteries** | Multi-DB ORM with Eager Loading, Redis Sentinel/Cluster support, Background Jobs (Redis/SQL) |
| **🛠️ DX & Tooling** | High-fidelity Code Scaffolding, TypeScript Client Generation, Interactive REPL |
| **🌐 Web & SSR** | Native Vite Integration, Asset Helpers, Flash Messages, WebSocket Rooms |
| **📈 Ops Ready** | OpenTelemetry Tracing, Centralized Health/Readiness, Native Circuit Breakers |

## 🏁 Quickstart (Build an app in 5 minutes)

### 1. Install the CLI
```bash
go install github.com/astraframework/astra/cli/astra@latest
```

### 2. Scaffold your Project
```bash
astra new myapp --api-only
cd myapp
```

### 3. Setup & Start
```bash
# Setup dependencies and run migrations
make setup
astra dev
```

Your app is now running at `http://localhost:3333` with Auto-Reload enabled!

## 📖 Documentation

- **[Getting Started](docs/getting-started.md)** - From Zero to Production.
- **[Architecture Overview](docs/architecture.md)** - Understand the Core, IoC, and Providers.
- **[Routing & Middleware](docs/routing.md)** - Build elegant APIs with ease.
- **[Database & ORM](docs/orm.md)** - Fluent models, relationships, and migrations.
- **[Security Checklist](docs/security-checklist.md)** - Best practices for hardening your app.
- **[Deployment Guide](docs/deployment.md)** - Docker, Kubernetes, and Cloud providers.

## 🏗 Project Structure

Astra follows a canonical structure to keep your codebase clean and predictable:

```text
├── app/
│   ├── controllers/   # Request handlers
│   ├── models/        # Data models (ORM)
│   ├── jobs/          # Background tasks
│   └── listeners/     # Event listeners
├── config/            # Application configuration
├── database/
│   ├── migrations/    # Database schema versions
│   └── seeders/       # Test data
├── routes/            # Route definitions
├── storage/           # Local file storage
└── main.go            # Application entry point
```

## 🧪 Testing

Astra is built for testability. Use our high-fidelity Test Client for expressive integration tests:

```go
func TestUserCreation(t *testing.T) {
    app := testing.NewTestApp(t)
    
    app.POST("/users", `{"name": "John"}`).
        ExpectStatus(201).
        ExpectJSON("name", "John")
}
```

## 🤝 Contributing

We welcome contributions! Please see our **[Contributing Guide](docs/contributing.md)** for details.

## 📄 License

Astra is open-source software licensed under the **[MIT License](LICENSE)**.

