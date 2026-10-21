# Authentication and security

> Source files reviewed for this document: `pkg/identity/auth/*.go`,
> `pkg/identity/claims/claims.go`, `pkg/identity/rbac/rbac.go`,
> `pkg/engine/http/csrf.go`, `security.go`, `middleware_auth.go`,
> `middleware_rbac.go`, `pkg/crypto/encrypter.go`, `SECURITY.md`, and
> authentication tests.

## Security boundary

Astra supplies primitives and middleware. Your application still owns identity
storage, authorization policy design, secret management, and endpoint-specific
validation.

Do not use development secrets in production. Keep `.env` out of source
control, provide secrets through the deployment environment, and call
`AstraConfig.ValidateProduction` before serving traffic.

## JWT

`auth.NewJWTManager` issues and verifies HMAC-signed access and refresh token
pairs:

```go
tokens, err := manager.IssueTokenPair(ctx, userID, map[string]any{
    "role": "admin",
})
claims, err := manager.Verify(tokens.AccessToken)
```

JWT secrets must be at least 32 bytes. Multiple comma-separated secrets can
include key IDs (`current:secret,previous:secret`) for key rotation; the first
key is active for new tokens. Refresh tokens are marked with `typ=refresh` and
can be stored/revoked through Redis. A refresh token cannot be used as an
access token.

JWT expiry and issuer come from `config.AuthConfig`. Verify issuer and claims
appropriate to your service before authorizing sensitive actions.

## Passwords

`auth.HashPassword` and `auth.CheckPassword` use the package's password
hashing implementation. Never store or log plaintext passwords. See the
function documentation for the accepted password and hash formats.

## Sessions

`auth.SessionDriver` is a small interface for session data. Redis is provided
by `NewRedisSessionDriver`. The separate `pkg/session` package supplies cookie
stores and HTTP middleware used by the SSR example.

Session cookies require a strong `APP_KEY`. Configure cookie security for the
deployment's HTTPS topology and use secure, same-site settings in production.

## OAuth2 and PKCE

`OAuth2Provider` defines `Name`, `AuthURL`, `Exchange`, and `UserInfo`.
`OAuth2Manager` registers providers explicitly and manages cryptographic state,
stored for ten minutes in Redis when a client is configured:

```go
state, err := manager.GenerateState(ctx)
if err != nil {
    return err
}
// Redirect to provider.AuthURL(state, map[string]string{
//     "code_challenge": challenge,
//     "code_challenge_method": "S256",
// })
```

The package includes adapters for Google, GitHub, Apple, Discord, and
Microsoft. Verify the state exactly once on callback. Use PKCE for public
clients and do not expose client secrets to browsers.

The guard helpers in `pkg/identity/auth/guard.go` also expose a package-level
named guard registry. That registry exists in the current implementation for
compatibility; new code should prefer constructing a guard and passing it
directly to `http.Auth`.

## TOTP

`TOTPManager.Generate` returns a secret and a base64-encoded PNG QR code.
`Verify` validates a submitted code. Treat the secret as a credential and
store it encrypted.

## RBAC and policies

`pkg/identity/rbac` is storage-agnostic. Implement its `Storage` interface,
then construct:

```go
rbac := rbac.NewRBAC(storage, cache, logger)
result, err := rbac.CheckAccess(ctx, &rbac.AccessRequest{
    UserID: "user-123",
    Resource: "orders",
    Action: "read",
})
```

It models permissions, roles, user-role assignments, and policies. `Allows`
provides a boolean convenience method using `AuthClaims`.

Authorization should be deny-by-default. Check tenant/resource ownership in
the request context and policy conditions; authentication alone is not
authorization.

## HTTP security

The HTTP package includes:

- `SecureHeaders` for browser security headers
- CSRF middleware for state-changing requests
- CORS configuration
- request body limits
- rate limiting
- timeout and recovery middleware
- authentication and RBAC middleware
- upload validation and size limits

Register only the middleware required by the service and configure trusted
proxies carefully. Do not trust client-supplied forwarding headers unless the
network boundary is controlled.

## Encryption

`crypto.NewEncrypter` implements AES-256-GCM and requires exactly 32 key bytes:

```go
enc, err := crypto.NewEncrypter(os.Getenv("APP_ENCRYPTION_KEY"))
sealed, err := enc.Encrypt("sensitive value")
plain, err := enc.Decrypt(sealed)
```

The package also exposes `InitDefaultEncrypter` and `DefaultEncrypter` for
legacy transparent-field integration. Prefer an explicitly injected
`*crypto.Encrypter` in new code because global mutable state complicates tests.

## Reporting vulnerabilities

Do not open a public issue for a security vulnerability. Follow
[`SECURITY.md`](../SECURITY.md), which requests a report to
`security@astra.dev`.
