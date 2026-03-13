# Testing Guide

Astra treats testing as a first-class citizen. Every application scaffolded with Astra comes with a robust testing suite ready to go.

## The `testing` Package

Astra provides advanced helpers in the `github.com/astraframework/astra/testing` package to make testing your application delightful.

## HTTP Integration Testing

Use the `TestApp` and its fluent client to test your API endpoints.

```go
func TestHealthCheck(t *testing.T) {
    // Setup a new test application
    app := testing.NewTestApp(t, func(app *core.App, r *http.Router) {
        routes.Register(r)
    })

    // Perform a request and assert the response
    app.GET("/health").
        AssertStatus(200).
        AssertJSON("status", "ok")
}
```

### Authenticated Requests

Testing protected routes is easy with the `WithAuth` helper.

```go
func TestProfile(t *testing.T) {
    app := testing.NewTestApp(t)
    token := "your-jwt-token"

    app.WithAuth(token).
        GET("/api/profile").
        AssertStatus(200)
}
```

## Database Testing

Astra makes it easy to test database interactions without polluting your actual database.

### Transactional Tests (Coming Soon)
We recommend using database transactions for your tests. Astra will soon provide a helper that automatically wraps each test in a transaction and rolls it back when the test finishes.

### Seeders
Use seeders to populate your test database with known data.

```go
func TestUserSearch(t *testing.T) {
    app := testing.NewTestApp(t)
    orm.Seed(&UserSeeder{}) // Run your seeder

    app.GET("/users?q=John").
        AssertJSONCount("data", 1)
}
```

## Mocking Services

Since Astra uses an IoC container, you can easily swap out real services for mocks during testing.

```go
app.Container().Bind("mailer", func() any {
    return &MockMailer{}
})
```
