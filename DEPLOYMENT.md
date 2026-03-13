# Deploying Astra Applications

Astra applications compile down to a single static binary containing your complete web stack, meaning the deployment process is extremely straightforward and robust.

## Prerequisites

Regardless of where you deploy Astra, you will need the following environment variables available at runtime:

- `APP_ENV=production` (Critical: Enables JSON logging, disables dev tools, enables secure cookies)
- `APP_KEY=your-32-byte-secure-string` (Used for encrypting session cookies)
- `JWT_SECRET=primary-secret,legacy-secret` (Supports comma-separated secrets for rotation)
- `PORT=3333` (Or dynamically provided by your host)

## Distributed Systems & Scaling

Astra is designed for multi-node deployments (e.g., Kubernetes, AWS ECS).

### Migration Locking
In a multi-node environment, multiple instances starting simultaneously might attempt to run migrations at the same time. Astra automatically handles this using **DB-native Advisory Locks**:
- **PostgreSQL**: Uses `pg_advisory_lock` to ensure only one node runs migrations.
- **MySQL**: Uses `GET_LOCK`.
- **SQLite**: No locking applied (safe for single-file access).

No extra configuration is required; Astra's migration runner is multi-node safe by default.

### JWT Secret Rotation
To rotate your JWT secrets without invalidating all active user sessions:
1. Generate a new secret.
2. Update `JWT_SECRET` to include both the new and old secrets, separated by a comma: `JWT_SECRET=new-secret,old-secret`.
3. Astra will sign new tokens using the first secret (`new-secret`) and attempt verification against all provided secrets.
4. Once old tokens have expired, you can remove `old-secret` from the list.

Astra automatically assigns a Key ID (`kid`) to tokens to speed up verification.

## Compile the Application

Building an Astra application for production requires disabling CGO for a pure, statically-linked binary (ideal for Alpine or Scratch Docker containers).

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server main.go
```

## Docker Deployment

The `astra new` CLI creates a production-ready, multi-stage `Dockerfile` by default. 
It uses `golang:alpine` to build the binary, and then copies only the binary into an empty `scratch` image. This results in an incredibly small container image (~20MB total length).

### Example `docker-compose.yml` for Production

```yaml
version: '3.8'
services:
  web:
    image: my-astra-app:latest
    restart: always
    ports:
      - "3333:3333"
    environment:
      - APP_ENV=production
      - APP_KEY=${APP_KEY}
      - DATABASE_URL=postgres://user:pass@db:5432/db_name?sslmode=disable
      - REDIS_URL=redis://cache:6379/0
    depends_on:
      - db
      - cache

  db:
      # your database setup...

  cache:
      # your redis setup...
```

## Render.com Deployment

Astra is perfectly suited to PaaS providers like Render or Heroku. 

Because `astra new` scaffolds a `go.mod`, Render will automatically detect it as a Go Web Service.
Simply provide the required Environment Variables in the Render dashboard, and configure the Start Command:

- **Build Command:** `go build -o server main.go`
- **Start Command:** `./server`

## Deploying on a VM (Systemd)

If you are managing your own servers (e.g., DigitalOcean, Hetzner), you can run Astra directly using a `systemd` service.

1. Create a service file: `/etc/systemd/system/astra.service`

```ini
[Unit]
Description=Astra Web Application
After=network.target postgresql.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/var/www/my-app
ExecStart=/var/www/my-app/server
Restart=always
RestartSec=3

Environment=APP_ENV=production
Environment=APP_KEY=your-production-key
Environment=PORT=8080
# Or use an env file:
# EnvironmentFile=/var/www/my-app/.env

[Install]
WantedBy=multi-user.target
```

2. Reload and start:
```bash
sudo systemctl daemon-reload
sudo systemctl start astra
sudo systemctl enable astra
```

## Reverse Proxy (Nginx / Caddy)

Astra applications typically run behind a reverse proxy that acts as the TLS terminator.

### Nginx Example

```nginx
server {
    listen 80;
    server_name myapp.com;

    location / {
        proxy_pass http://127.0.0.1:3333;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_addres_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Astra's internal helpers (like `c.ClientIP()`) correctly parse standard proxy headers (`X-Forwarded-For` and `X-Real-Ip`) to ensure you see the actual client IP, not the Nginx IP.
