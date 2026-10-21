// Package auth provides authentication building blocks for Go applications.
// It supports multiple authentication strategies, password hashing, JWT
// management, and OAuth2 integration.
//
// Core Features:
//   - Authentication strategies: JWT and session-based strategies.
//   - Password hashing: Argon2ID password hashing.
//   - Tokens: JWT generation, signing, and verification.
//   - OAuth2: Adapters for social login providers.
//   - Auditing: Audit logging for security events.
//
// Construct the dependencies used by an application and pass them to its
// handlers and middleware.
package auth
