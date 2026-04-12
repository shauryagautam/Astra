# 03. Security

Security in Astra is layered. Authentication tells you who the caller is. Authorization tells you what that caller may do. Password hashing protects credentials at rest. Policies keep resource-level decisions out of your handlers.

## Why this split matters

If you mix authentication and authorization together, every handler becomes a special case. Astra keeps the surface area explicit: guards identify users, middleware enforces coarse permissions, and policies handle resource-specific decisions.

## The unified guard API

Astra uses the `auth.Guard` interface to abstract how a user authenticates. Both `JWTGuard` and `CookieGuard` implement the same behavior contract:

```go
type Guard interface {
    Name() string
    Authenticate(c *http.Context) (any, error)
    Login(c *http.Context, user any) error
    Logout(c *http.Context) error
}
```

- **Authenticate**: Extract the user from the request (header or cookie).
- **Login/Logout**: Handle session lifecycle for stateful guards.

Use `JWTGuard` when you want stateless API auth. Use `CookieGuard` when you want browser sessions with secure cookies and server-side session state.

The package also keeps a registry of guards by name:

```go
auth.Register("api", jwtGuard)
auth.Register("web", cookieGuard)

guard := auth.Resolve("web")
```

> [!TIP]
> Keep one guard per concern. The browser session and the API token should not share the same identity strategy unless you have a strong reason to do so.

## Password hashing

Never store plain-text passwords. Astra gives you two safe paths:

1. `auth.HashPassword` and `auth.CheckPasswordHash` for a simple bcrypt-based flow.
2. `auth.NewArgon2idHasher()` when you want PHC-formatted hashes with explicit parameter control and rehash detection.

The important part is not the algorithm branding. The important part is that the hashing work is performed before persistence and that verification uses a constant-time comparison path.

> [!NOTE]
> `HashPassword` uses bcrypt with cost 12. `NewArgon2idHasher()` is available when you want an Argon2id configuration that can evolve over time.

## RBAC middleware

Use RBAC when the question is coarse-grained: can this caller access this endpoint or perform this action at all? 

Register the middleware in your router:

```go
r.Get("/admin", adminHandler.Index).
    Middleware(auth.RBAC("admin", "editor"))
```

Astra’s RBAC middleware turns the authenticated user into an access request and fails fast with a `403` when permission is missing. This keeps the handler focused on business logic instead of permission plumbing.

## Policies and resource-level authorization

RBAC is not enough for rules like “a user can edit only their own post.” For that, Astra’s `pkg/policy` package provides policy functions and scope functions.

Use `policy.Register` to answer yes/no questions for a resource. Use `policy.RegisterScope` to inject resource-scoping into query builders so the data layer enforces the same rule consistently.

This is the right place for resource ownership checks, tenant isolation, and query-level filtering.

## Why policies are better than ad hoc checks

Policy rules belong at the boundary because they should be reused by handlers, background jobs, and query scopes. If the same rule appears in five handlers, it is already a shared policy and should be treated like one.

> [!WARNING]
> Deny by default. If no policy is registered, the request should not be treated as implicitly safe.

## Copy-Paste Example

```go
package main

import (
	"context"
	"log/slog"

	"github.com/shauryagautam/Astra/pkg/engine/http"
	"github.com/shauryagautam/Astra/pkg/identity/auth"
	"github.com/shauryagautam/Astra/pkg/policy"
)

type Post struct {
	ID     string
	UserID string
}

type User struct {
	ID    string
	Roles []string
}

func main() {
	logger := slog.Default()
	_ = logger

	jwtGuard := auth.NewJWTGuard("api", mustJWTManager())
	cookieGuard := auth.NewCookieGuard("web", mustSessionStore())
	auth.Register(jwtGuard.Name(), jwtGuard)
	auth.Register(cookieGuard.Name(), cookieGuard)

	policy.Register("update", (*Post)(nil), func(user any, subject any) bool {
		u := user.(*User)
		p := subject.(*Post)
		return u.ID == p.UserID || hasRole(u.Roles, "admin")
	})

	policy.RegisterScope((*Post)(nil), func(ctx context.Context, builder any) {
		_ = ctx
		_ = builder
		// Inject tenant or ownership filters here.
	})

	_ = auth.HashPassword
	_ = http.RateLimit
}

func hasRole(roles []string, role string) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}
	return false
}

func mustJWTManager() *auth.JWTManager { return nil }
func mustSessionStore() auth.SessionDriver { return nil }
```

---

**Next Chapter: [04. Persistence](./04-persistence.md)**
