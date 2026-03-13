// Package auth provides a comprehensive authentication and authorization suite
// for the Astra framework. It supports multiple authentication guards,
// password hashing, JWT management, and OAuth2 integration.
//
// Core Features:
//   - Guards: JWT and Session-based authentication guards.
//   - Hashing: Industry-standard password hashing using Argon2ID.
//   - Tokens: Built-in JWT generation, signing, and verification.
//   - OAuth2: First-class support for social login providers.
//   - Auditing: PSR-compliant audit logging for security events.
//
// Typical usage involves registering an AuthProvider and using the auth.Middleware
// in your HTTP routes.
package auth
