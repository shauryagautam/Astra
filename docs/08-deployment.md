# 08. Deployment

Shipping Astra to production is about producing one runnable artifact that already contains the frontend assets, the compiled Go binary, and a clean shutdown path.

## Why deployment needs to be planned early

If you wait until the end to think about build steps and shutdown signals, you usually end up with a release process that is fragile, slow, or both. Astra’s deployment story is intentionally boring: build the frontend, build the binary, package them together, and let the app stop itself cleanly when the platform asks.

## Build the frontend and backend together

The frontend build generates the manifest and fingerprinted assets that the Go app uses in production. The Go build compiles the server with the same code path you use locally, just with production flags.

The important part is the order:

1. Build frontend assets first.
2. Build the Go binary with `-trimpath` and stripped symbols.
3. Copy both artifacts into the final runtime image.

## Multi-stage Dockerfile

Astra works well with a multi-stage Dockerfile because the final image only needs the compiled binary and the frontend output.

```dockerfile
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/astra .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/astra ./astra
COPY --from=frontend /src/frontend/dist ./frontend/dist
EXPOSE 3333
ENTRYPOINT ["/app/astra"]
```

> [!TIP]
> Use a minimal runtime image. The smaller the final image, the smaller the attack surface and the faster the pull time.

## Graceful shutdowns

Astra’s `App` listens for `SIGINT` and `SIGTERM`, then runs `OnStop` hooks in reverse order. That makes the shutdown sequence safe for container platforms, load balancers, and rolling updates.

Use `OnStop` for anything that must close cleanly: database pools, Redis clients, queue workers, tracing exporters, and buffered logs.

## Copy-Paste Example

```bash
#!/bin/sh
set -e

./astra db:migrate
exec ./astra
```

```go
app.OnStop(func(ctx context.Context) error {
	return db.Close()
})
```

---

**Back to: [Architecture Overview](../README.md#documentation)**
