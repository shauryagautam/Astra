# SSR Auth Example

This example demonstrates a full-stack, Server-Side Rendered (SSR) application with integrated authentication and session management built using the [Astra Framework](https://github.com/shauryagautam/Astra).

## ✨ Features Demonstrated

- **SSR Template Engine**: Native integration with Astra's template engine for dynamic HTML rendering.
- **Session-based Authentication**: Secure session management using encrypted cookie stores.
- **Flash Messages**: Persistence of user feedback across redirects using session storage.
- **Middleware Orchestration**: Global and route-specific middleware registration (Auth guards).
- **Static Assets**: Handling of CSS/JS and public resources.

## 🚀 Quickstart

### 1. Start Infrastructure
Launch required services (Redis/Database) using Docker:
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
Test the endpoint:
```bash
curl http://localhost:3333/
```

## 🏗 Key Components

- **`views/`**: HTML templates organized by layout and component.
- **`routes/`**: Route definitions including authentication middleware wrapping.
- **`main.go`**: Bootstrapping session providers, template engines, and the HTTP router.
- **`handler/`**: Controllers that manage session state and render views.

---
Built with ❤️ using [Astra](https://github.com/shauryagautam/Astra)
