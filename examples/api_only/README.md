# API Only Example

This is a high-fidelity, production-grade API implementation demonstration built with the [Astra Framework](https://github.com/shauryagautam/Astra). It focuses on illustrating how to build clean, JSON-only services with robust validation and data persistence.

## ✨ Features Demonstrated

- **Lean REST API**: Minimalist handlers for resource management (CRUD).
- **Automated Schema Initialization**: Demonstrates `app.OnStart` hooks for database preparation.
- **Custom Providers**: Shows how to wrap external libraries (like `validate`) into the Astra IoC container.
- **Postgres Integration**: Native use of the Astra ORM and database providers.
- **Operational Ready**: Pre-configured with Docker Compose and `.env` handling.

## 🚀 Quickstart

### 1. Start Infrastructure
Launch the required database using Docker:
```bash
docker-compose up -d
```

### 2. Configure Environment
Copy the example environment file:
```bash
cp .env.example .env
```

### 3. Run the Application
Start the Go server:
```bash
go run main.go
```

### 4. Verify
Test the endpoint (Wait for "Todos table ready!" message):
```bash
curl http://localhost:3333/ping
```

## 🏗 Key Components

- **`main.go`**: App bootstrapping, provider registration, and lifecycle management.
- **`routes/`**: Clean route definitions separated from application logic.
- **`handlers/`**: Request processing logic.
- **`schema/`**: Type definitions for requests and responses.

---
Built with ❤️ using [Astra](https://github.com/shauryagautam/Astra)
