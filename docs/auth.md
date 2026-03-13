# Authentication & Authorization

Astra provides a multi-guard authentication system and a powerful authorization gate (Bouncer) to secure your applications.

## Authentication Guards

Astra supports multiple "guards" for identifying users.

### JWT Guard

Used for stateless APIs and mobile backends. Tokens are typically sent in the `Authorization: Bearer <token>` header.

### Session Guard

Used for stateful web applications (SSR). Uses secure, encrypted cookies to track users.

## Registering the Auth Provider

In your `main.go`:

```go
app.Use(auth.NewAuthProvider(auth.Config{
    DefaultGuard: "jwt",
    JWTSecret:    os.Getenv("JWT_SECRET"),
}))
```

## Guarding Routes

Use the `auth.Middleware` to protect your routes:

```go
r.Group("/dashboard", func(r *http.Router) {
    r.Use(auth.Middleware()) // Requires authentication
    r.Get("/", DashboardHandler)
})
```

## Authorization (Bouncer)

Astra uses "Policies" to define authorization logic.

### Defining a Policy

```go
type PostPolicy struct{}

func (p *PostPolicy) Update(user *User, post *Post) bool {
    return user.ID == post.UserID
}
```

### Checking Permissions

Inside a handler:

```go
func UpdatePost(c *http.Context) error {
    post := ...
    if err := c.Authorize("update", post); err != nil {
        return err // 403 Forbidden
    }
    // ... update logic
}
```

## Security Defaults

- **Password Hashing**: Astra uses Argon2ID with secure defaults.
- **Session Security**: Sessions use `HttpOnly`, `Secure`, and `SameSite: Lax` cookies by default.
- **JWT Signing**: Uses HMAC SHA-256 with 32-byte minimum keys.
