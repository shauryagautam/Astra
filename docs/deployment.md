# Deployment Guide

Astra applications are standard Go binaries, making them easy to deploy across various environments including Bare Metal, Docker, and Kubernetes.

## 1. Preparing for Production

Before deploying, ensure your environment is configured correctly.

### Essential Env Variables

| Variable | Description |
| --- | --- |
| `APP_ENV` | Set to `production`. |
| `APP_KEY` | High-entropy string for session/encryption. |
| `APP_DEBUG` | Set to `false`. |
| `DATABASE_URL` | Production database connection string. |
| `JWT_SECRET` | Secret key for signing tokens. |

## 2. Docker Deployment

Astra includes a production-ready `Dockerfile` by default in every new project.

### Multi-stage Build
Our default Dockerfile uses a multi-stage build to keep the final image size minimal and secure.

```dockerfile
# Build Stage
FROM golang:1.22-alpine AS builder
...
# Production Stage
FROM alpine:latest
...
```

### Running with Docker Compose
For simpler deployments, you can use the provided `docker-compose.yml`.

```bash
docker-compose up -d --build
```

## 3. Kubernetes Deployment

When deploying to K8s, take advantage of Astra's built-in health checks.

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 3333
readinessProbe:
  httpGet:
    path: /ready
    port: 3333
```

## 4. Database Migrations

In production, run migrations as part of your CI/CD pipeline or as a pre-start step in your container.

```bash
# Run migrations to latest version
./myapp migrate up
```

## 5. Observability

Astra integrates with **OpenTelemetry** out of the box. Simply provide an OTLP endpoint:

```env
OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4317
OTEL_SERVICE_NAME=my-astra-app
```

Tracing, metrics, and logs will be automatically propagated.
