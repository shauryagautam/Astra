# Production Readiness Checklist

Before going live with your Astra application, go through this checklist to ensure maximum security, performance, and reliability.

## 🛡️ Security

- [ ] **APP_KEY**: Is it set to a cryptographically secure 32-byte string?
- [ ] **DEBUG**: Is `APP_DEBUG` set to `false`?
- [ ] **HTTPS**: Are you enforcing SSL/TLS? (Astra does this by default if headers are present).
- [ ] **CSRF**: Is CSRF protection enabled for all state-changing routes?
- [ ] **Secure Headers**: Are standard secure headers (HSTS, CSP, XFO) configured in your router?
- [ ] **Rate Limiting**: Have you applied `Throttle` to sensitive endpoints (Login, Register)?
- [ ] **Database Credentials**: Are you using a restricted database user (no superuser/owner)?

## ⚡ Performance

- [ ] **Production Build**: Are you running the binary built with `go build -ldflags="-s -w"`?
- [ ] **Connection Pooling**: Are `DB_MAX_CONNS` and `DB_MAX_IDLE` tuned for your load?
- [ ] **Redis**: Is Redis configured for caching and sessions?
- [ ] **Vite Build**: If using SSR, has `npm run build` been executed and the manifest generated?

## 🩺 Reliability & Observability

- [ ] **Health Checks**: Are `/health` and `/ready` mapped in your load balancer?
- [ ] **OTLP Exporter**: Is `OTEL_EXPORTER_OTLP_ENDPOINT` set to your collector?
- [ ] **Logging**: Is `APP_LOG_LEVEL` set to `info` or `warn`? Is structured logging (JSON) enabled?
- [ ] **Graceful Shutdown**: Does your deployment (e.g., K8s) respect the `SIGTERM` signal?

## 📦 Deployment

- [ ] **Migrations**: Have you ran `astra migrate up` on your production database?
- [ ] **Secrets**: Are secrets managed via a secure vault or environment injection?
- [ ] **Backups**: Are you performing regular database and storage backups?
