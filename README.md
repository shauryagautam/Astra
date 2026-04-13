# Astra Framework

[**astraframework.appwrite.network**](https://astraframework.appwrite.network/)

**A batteries-included, production-grade Go web framework.**

[![Go Report Card](https://goreportcard.com/badge/github.com/shauryagautam/Astra)](https://goreportcard.com/report/github.com/shauryagautam/Astra) [![Go Reference](https://pkg.go.dev/badge/github.com/shauryagautam/Astra.svg)](https://pkg.go.dev/badge/github.com/shauryagautam/Astra) [![CI/CD Pipeline](https://github.com/shauryagautam/Astra/actions/workflows/pipeline.yml/badge.svg)](https://github.com/shauryagautam/Astra/actions/workflows/pipeline.yml) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

Astra provides everything you need to build scalable, secure, and maintainable applications with Go. It prioritizes **Developer Experience (DX)**, **Security by Default**, and **Operational Maturity**.

## 📍 Table of Contents

- [✨ Why Astra?](#why-astra)
- [🚀 Key Features](#key-features)
- [🏁 Quickstart](#quickstart)
- [📖 Documentation](#documentation)
- [🏗 Project Structure](#project-structure)
- [🧪 Testing](#testing)
- [🤝 Contributing](#contributing)
- [📄 License](#license)

---

## Why Astra?

Astra was built to solve the "configuration fatigue" in the Go ecosystem. While Go is powerful, setting up a production-ready web server often requires stitching together dozens of libraries. Astra gives you a cohesive, opinions-included foundation:

- **💨 Zero to Production**: Pre-configured with everything from database migrations to OpenTelemetry.
- **🎨 Modern Frontend**: First-class support for Vite, allowing you to build rich SSR or SPA apps seamlessly.
- **🛡️ Secure by Design**: Transparent encryption for sensitive data and automatic security headers.
- **📈 Scalable Architecture**: Built-in support for Redis Sentinel, Cluster, and multi-tenant background jobs.

## Key Features

| Category | Capabilities |
| --- | --- |
| **⚡️ Performance** | Multi-threaded Event Emitter, Bytedance Sonic JSON engine, high-speed routing |
| **🛡️ Security** | Transparent PII Encryption, PSR Audit Logging, PKCE OAuth2, CSP & HSTS middleware |
| **📦 Batteries** | Unified Migration Runner (Advisory Locking), Redis Sentinel, Job Queues |
| **🛠️ DX & Tooling** | High-fidelity Scaffolding, SSE-powered Dev Dashboard, Key Rotation CLI |
| **🌐 Web & SSR** | Native Vite Integration, Asset Helpers, Flash Messages, WebSocket Channels |
| **📈 Ops Ready** | OpenTelemetry, Prometheus (/metrics), Liveness/Readiness probes |

## Quickstart

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

Your app is now running at `http://localhost:3333` with **Auto-Reload** enabled!

## Documentation

- **[01. Foundation](docs/01-foundation.md)** - From Zero to Production.
- **[02. Architecture](docs/02-architecture.md)** - Understand the Core, IoC, and Providers.
- **[03. Security](docs/03-security.md)** - Guards, RBAC, and policies.
- **[04. Persistence](docs/04-persistence.md)** - Fluent models, relationships, and migrations.
- **[05. Frontend](docs/05-frontend.md)** - Vite integration, SSR, and assets.
- **[06. Observability](docs/06-observability.md)** - Logs, Traces, and Resilience.
- **[07. Testing](docs/07-testing.md)** - Expressive integration and unit tests.
- **[08. Deployment](docs/08-deployment.md)** - Docker, Kubernetes, and Cloud.

## Project Structure

Astra enforces a clean, predictable directory layout inspired by industry best practices:

```text
├── app/
│   ├── handler/       # Request handlers (Controllers)
│   ├── schema/        # Data models, validation & DTOs
│   ├── jobs/          # Background task logic
│   └── middleware/    # Application-specific middleware
├── database/
│   ├── migrations/    # Versioned SQL migration files
│   └── seeders/       # Test data sets for development
├── internal/          # Private core framework logic
├── pkg/               # Publicly exportable packages
├── routes/            # Route definitions & registration
├── shared/            # Shared types, constants, and clients
├── main.go            # Application entry point
└── wire.go            # Dependency injection source
```

## Testing

Astra is built for testability. Use our high-fidelity `test_util` package for expressive integration tests:

```go
func TestUserCreation(t *testing.T) {
    // 1. Initialize Test App with temporary resources
    app := test_util.NewTestApp(t, nil)
    
    // 2. Perform actions and assert results fluently
    app.POST("/users", map[string]string{"name": "John"}).
        ExpectStatus(201).
        ExpectJSON("name", "John")
}
```

## Contributing

We welcome contributions! Whether it's a bug report, feature request, or a PR, check out our **[Contributing Guide](CONTRIBUTING.md)** to get started.

## License

Astra is open-source software licensed under the **[MIT License](LICENSE)**.
