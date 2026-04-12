# Astra Framework

<p align="center">
  <img src="https://raw.githubusercontent.com/shauryagautam/Astra/main/assets/logo.png" width="200" alt="Astra Logo">
</p>

<p align="center">
  <b>A batteries-included, production-grade Go web framework inspired by AdonisJS and Ruby on Rails.</b>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/shauryagautam/Astra"><img src="https://goreportcard.com/badge/github.com/shauryagautam/Astra" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/shauryagautam/Astra"><img src="https://pkg.go.dev/badge/github.com/shauryagautam/Astra.svg" alt="Go Reference"></a>
  <a href="https://github.com/shauryagautam/Astra/actions/workflows/pipeline.yml"><img src="https://github.com/shauryagautam/Astra/actions/workflows/pipeline.yml/badge.svg" alt="CI/CD Pipeline"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</p>

---

Astra provides everything you need to build scalable, secure, and maintainable applications with Go. It prioritizes **Developer Experience (DX)**, **Security by Default**, and **Operational Maturity**.

## 🚀 Key Features

| Category | Capabilities |
| --- | --- |
| **⚡️ Performance** | Multi-threaded Event Emitter, Bytedance Sonic JSON, Cursor-based Pagination |
| **🛡️ Security** | Transparent PII Encryption, PSR Audit Logging, PKCE OAuth2, CSP Nonce Middleware |
| **📦 Batteries** | Unified Migration Runner (Advisory Locking), Redis Sentinel/Cluster, Background Jobs |
| **🛠️ DX & Tooling** | High-fidelity Scaffolding, Real-time Dev Dashboard (SSE), Key Rotation CLI |
| **🌐 Web & SSR** | Native Vite Integration, Asset Helpers, Flash Messages, WebSocket Channels |
| **📈 Ops Ready** | OpenTelemetry, Prometheus Metrics (/metrics), Centralized Health/Readiness |

## 🏁 Quickstart (Build an app in 5 minutes)

### 1. Install the CLI

```bash
go install github.com/shauryagautam/Astra/cmd/astra@latest
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

- **[01. Foundation](docs/01-foundation.md)** - From Zero to Production.
- **[02. Architecture](docs/02-architecture.md)** - Understand the Core, IoC, and Providers.
- **[03. Security](docs/03-security.md)** - Guards, RBAC, and policies.
- **[04. Persistence](docs/04-persistence.md)** - Fluent models, relationships, and migrations.
- **[06. Observability](docs/06-observability.md)** - Logs, Traces, and Resilience.
- **[08. Deployment](docs/08-deployment.md)** - Docker, Kubernetes, and Cloud.

## 🏗 Project Structure

Astra follows a clean, explicit structure to keep your codebase predictable:

```text
├── app/
│   ├── handler/       # Request handlers
│   ├── schema/        # Data models & validation
│   └── jobs/          # Background tasks
├── database/
│   ├── migrations/    # SQL migration files
│   └── seeders/       # Test data sets
├── routes/            # Route definitions
├── shared/            # Shared types and clients
├── main.go            # Application entry point
└── wire.go            # Dependency injection source
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

We welcome contributions! Please see our **[Contributing Guide](CONTRIBUTING.md)** for details.

## 📄 License

Astra is open-source software licensed under the **[MIT License](LICENSE)**.

