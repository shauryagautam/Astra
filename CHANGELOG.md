# Changelog

All notable changes to Astra are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [1.0.0] - 2026-05-19

### Added
- Initial stable release of the Astra Go web framework
- HTTP engine with middleware chain, rate limiting, CSRF, CORS, and security headers
- ORM with query builder, relationships, migrations, and multi-database support
- Identity system: JWT, session, OAuth2 (Google, GitHub, Apple, Discord, Microsoft), TOTP
- RBAC and policy-based authorization
- Redis integration: pub/sub, streams, distributed locks, rate limiting
- Background job queue with Redis backend, scheduler, and failed-job tracking
- WebSocket and SSE real-time support
- OpenTelemetry tracing and Prometheus metrics
- GraphQL support with dataloaders and directives
- S3 and local file storage
- Mail system: SMTP, Resend, queue-backed sending
- i18n support
- Dependency injection via Google Wire
- Test utilities: TestApp, HTTP assertions, factory system, mock dispatcher
- Benchmarks for routing and JSON encoding
- Docker and Fly.io deployment templates
- Examples: api_only, ssr_auth, orm_demo, multi_database
