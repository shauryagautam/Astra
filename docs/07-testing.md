# 07. Testing

Testing in Astra is built around the same principle as the runtime: explicit dependencies, visible behavior, and as little hidden magic as possible.

## Why testing is a first-class concern

The fastest tests are the ones that do not need to guess how the app was assembled. Astra’s dependency graph is explicit, which means you can replace a real provider with a fake provider, then exercise the app through the same boundaries production uses.

## HTTP testing with `NewTestApp`

The primary helper lives in `test_util`. Most teams import it with an alias such as `astratest` so the test code reads cleanly:

```go
app := astratest.NewTestApp(t, func(app *engine.App, r *astrahttp.Router) {
	r.Get("/health", healthHandler.ServeHTTP)
})
```

The client then gives you fluent assertions such as `ExpectStatus`, `ExpectJSON`, `ExpectHeader`, and `ExpectBodyContains`.

That combination makes handler tests read like intent instead of plumbing.

> [!TIP]
> Prefer end-to-end HTTP tests for behavior and small unit tests for branch-heavy helpers. You do not need to choose only one style.

## Mocking with Wire

Wire makes provider replacement straightforward. In a test-only injector, you can build the app with a mock mailer, a fake queue, or an in-memory storage backend instead of the production provider.

The important part is that the replacement happens at composition time, not by reaching into the app during the test.

## Real database tests with testcontainers-go

Astra’s `test_util.Suite` starts real Postgres and Redis containers using testcontainers-go, then wires the app against those live dependencies. That is the right default when you need to validate SQL behavior, advisory locks, transactions, Redis scripts, or other integration-sensitive paths.

Because the container lifecycle is part of the suite, the tests stay isolated and repeatable.

> [!WARNING]
> If the feature depends on real Postgres behavior, test it against real Postgres. Do not assume an in-memory substitute will behave the same way.

## Copy-Paste Example

```go
package users_test

import (
	"testing"

	"github.com/shauryagautam/Astra/pkg/engine"
	astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
	astratest "github.com/shauryagautam/Astra/test_util"
)

func TestCreateUser(t *testing.T) {
	app := astratest.NewTestApp(t, func(app *engine.App, r *astrahttp.Router) {
		r.Post("/users", func(c *astrahttp.Context) error {
			return c.JSON(map[string]any{"name": "John"}, 201)
		})
	})

	app.POST("/users", map[string]string{"name": "John"}).
		ExpectStatus(201).
		ExpectJSON("name", "John")
}

func TestRepositoryWithContainers(t *testing.T) {
	var suite astratest.Suite
	suite.SetupSuite()
	defer suite.TearDownSuite()
	_ = suite
}
```

---

**Next Chapter: [08. Deployment](./08-deployment.md)**
