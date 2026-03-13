# Routing & Middleware

Astra provides a fluent, type-safe routing API built on top of the battle-tested `go-chi` router.

## Defining Routes

Routes are typically defined in `routes/api.go` or `routes/web.go`.

```go
func Register(r *http.Router) {
    r.Get("/", func(c *http.Context) error {
        return c.SendString("Hello Astra")
    })

    r.Post("/users", "UserController@Store") // Resource-style resolution
}
```

## Route Parameters

You can capture segments of the URL using the `{}` syntax.

```go
r.Get("/users/{id}", func(c *http.Context) error {
    id := c.Param("id")
    return c.JSON(map[string]string{"id": id})
})
```

## Middleware

Middleware are functions that wrap your handlers. They can be applied globally, to groups, or to individual routes.

### Global Middleware
Global middleware runs on every request. Register them in your application entry point.

```go
router.Use(http.Logger())
router.Use(http.Recovery())
```

### Route Groups
Group routes to apply common middleware or prefixes.

```go
r.Group(func(r *http.Router) {
    r.Use(auth.Middleware()) // Apply auth to all routes in this group
    
    r.Get("/profile", "ProfileController@Show")
    r.Put("/profile", "ProfileController@Update")
}, "/api/v1")
```

## The Astra Context (`http.Context`)

Every handler receives an `http.Context`, which provides a powerful API for interacting with the request and response.

### Binding Request Data
```go
type CreateUserRequest struct {
    Email string `validate:"required,email"`
    Name  string `validate:"required"`
}

func Store(c *http.Context) error {
    var req CreateUserRequest
    if err := c.BindAndValidate(&req); err != nil {
        return c.ValidationError(err)
    }
    // ...
}
```

### Sending Responses
```go
c.JSON(data)           // 200 OK with JSON
c.Created(data)        // 201 Created
c.NoContent()         // 204 No Content
c.Error(404, "Lost")   // Error response with unique Error ID
```
