# Architecture Overview

Astra is built on a modular, provider-based architecture inspired by modern web framework best practices. It balances flexibility with a strong, opinionated core.

## The Core Philosophy

1.  **Inversion of Control (IoC)**: Everything in Astra is registered and resolved from a central Container.
2.  **Service Providers**: Modules are integrated into the application through simple, lifecycle-aware providers.
3.  **Explicit over Implicit**: We avoid "magic" wherever possible. Dependencies are resolved explicitly, and errors are handled first-class.

## Application Lifecycle

Every Astra application follows a predictable boot sequence:

1.  **Initialize**: `core.New()` creates the application instance and initializes the IoC container.
2.  **Register**: Service Providers are added via `app.Use()`. Each provider's `Register` method is called to bind services to the container.
3.  **Boot**: `app.Start()` is called. This triggers the `Boot` method of all registered providers in order.
4.  **Listen**: The HTTP server starts, and the application begins handling requests.

## The Container (IoC)

The Container is the heart of Astra. It stores all singleton services (Database, Redis, Mailer, etc.).

```go
// Resolving a service from the container
db := app.Get("orm").(*orm.Database)
```

## Service Providers

A Service Provider is a simple struct that implements the `ServiceProvider` interface:

```go
type MyProvider struct {}

func (p *MyProvider) Register(c *container.Container) {
    c.Bind("myservice", func() any {
        return &MyService{}
    })
}

func (p *MyProvider) Boot(c *container.Container) error {
    // Perform any startup logic
    return nil
}
```

## Request Lifecycle

1.  **TCP Connection**: Handled by the standard `net/http` server.
2.  **Middleware Stack**: The request passes through global and route-specific middleware.
3.  **Router**: `go-chi` matches the request to a handler.
4.  **Astra Context**: The request/response pair is wrapped in an `http.Context`.
5.  **Handler**: Your controller logic executes.
6.  **Response**: The `http.Context` methods (`JSON`, `HTML`, etc.) finalize the response.
