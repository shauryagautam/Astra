package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func writeFile(path string, content string) {
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0644)
}

func writeGitKeep(dir string) {
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644)
}

func NewCmd() *cobra.Command {
	var apiOnly, web, mobile bool

	cmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Create a new Astra application",
		Run: func(cmd *cobra.Command, args []string) {
			name := ""
			if len(args) > 0 {
				name = args[0]
			} else {
				prompt := promptui.Prompt{Label: "Project Name", Default: "astra-app"}
				var err error
				name, err = prompt.Run()
				if err != nil {
					fmt.Printf("Prompt failed %v\n", err)
					return
				}
			}

			if !apiOnly && !web && !mobile {
				prompt := promptui.Select{
					Label: "Select Project Type",
					Items: []string{"API Only", "Web (React + Vite)", "Mobile (Expo)", "Full Stack (Web + Mobile)"},
				}
				_, result, err := prompt.Run()
				if err != nil {
					fmt.Printf("Prompt failed %v\n", err)
					return
				}
				switch result {
				case "API Only":
					apiOnly = true
				case "Web (React + Vite)":
					web = true
				case "Mobile (Expo)":
					mobile = true
				case "Full Stack (Web + Mobile)":
					web = true
					mobile = true
				}
			}
			fmt.Println("Scaffolding Astra application...")

			// Standard Directories
			writeGitKeep(filepath.Join(name, "app/controllers"))
			writeGitKeep(filepath.Join(name, "app/models"))
			writeGitKeep(filepath.Join(name, "app/jobs"))
			writeGitKeep(filepath.Join(name, "database/migrations"))
			writeGitKeep(filepath.Join(name, "database/seeders"))
			writeGitKeep(filepath.Join(name, "shared/astra-client")) // Shared TS Client

			// main.go
			writeFile(filepath.Join(name, "main.go"), `
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/http"
	"`+name+`/routes"
)

func main() {
	app, err := core.New()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	app.OnStart(func(ctx context.Context) error {
		router := http.NewRouter(app)
		routes.Register(router)
		
		serverAddr := fmt.Sprintf("%s:%d", app.Config.App.Host, app.Config.App.Port)
		app.Logger.Info("HTTP server starting", "addr", serverAddr)
		
		go func() {
			http.ListenAndServe(serverAddr, router.Handler())
		}()
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatalf("App failed: %v", err)
	}
}
`)

			// go.mod
			writeFile(filepath.Join(name, "go.mod"), `
module `+name+`

go 1.22
`)

			// routes/api.go
			writeFile(filepath.Join(name, "routes", "api.go"), `
package routes

import (
	"github.com/astraframework/astra/http"
)

func Register(r *http.Router) {
	r.Get("/ping", func(c *http.Context) error {
		return c.JSON(map[string]string{"message": "pong"})
	})
	r.Get("/ready", func(c *http.Context) error {
		return c.SendString("OK")
	})
}
`)

			// .env and .env.example
			envContent := `APP_NAME=` + name + `
APP_ENV=development
APP_KEY=
APP_DEBUG=true
PORT=3333
HOST=0.0.0.0

DATABASE_URL=postgres://postgres:password@127.0.0.1:5432/` + name + `?sslmode=disable
DB_MAX_CONNS=10
DB_MIN_CONNS=2
DB_MAX_IDLE=30s
DB_SSL=disable

REDIS_URL=redis://127.0.0.1:6379/0

JWT_SECRET=supersecretjwtkey32bytesminimum
JWT_ISSUER=astra
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

MAIL_DRIVER=log
SMTP_HOST=127.0.0.1
SMTP_PORT=1025
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM=noreply@example.com

STORAGE_DRIVER=local
STORAGE_LOCAL_ROOT=./storage

QUEUE_DRIVER=redis
QUEUE_CONCURRENCY=5
QUEUE_PREFIX=astra:queue:

OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_SERVICE_NAME=` + name + `

WS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
`
			writeFile(filepath.Join(name, ".env.example"), envContent)
			writeFile(filepath.Join(name, ".env"), envContent)

			// .gitignore
			writeFile(filepath.Join(name, ".gitignore"), `
# Binaries
bin/
main
`+name+`

# Environment
.env

# Vendor
vendor/

# IDEs
.idea/
.vscode/

# OS
.DS_Store

# Go
coverage.out
`)

			// Dockerfile
			writeFile(filepath.Join(name, "Dockerfile"), `
# Build Stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# Production Stage
FROM scratch
WORKDIR /
COPY --from=builder /app/server /server
EXPOSE 3333
ENTRYPOINT ["/server"]
`)

			// docker-compose.yml
			writeFile(filepath.Join(name, "docker-compose.yml"), `
version: '3.8'
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
      POSTGRES_DB: `+name+`
    ports:
      - "5432:5432"
    volumes:
      - db_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

volumes:
  db_data:
  redis_data:
`)

			// fly.toml
			writeFile(filepath.Join(name, "fly.toml"), `
app = "`+name+`"
primary_region = "iad"

[build]
  dockerfile = "Dockerfile"

[http_service]
  internal_port = 3333
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 0
  processes = ["app"]

[[http_service.checks]]
  grace_period = "10s"
  interval = "15s"
  timeout = "2s"
  method = "GET"
  path = "/ready"
  protocol = "http"
`)

			// README.md
			writeFile(filepath.Join(name, "README.md"), `
# `+name+`

Built with the [Astra](https://github.com/astraframework/astra) framework.

## Quickstart

1. Start dependencies:
   `+"```bash"+`
   docker-compose up -d
   `+"```"+`

2. Run the application:
   `+"```bash"+`
   go run main.go
   `+"```"+`

3. Verify:
   `+"```bash"+`
   curl http://localhost:3333/ping
   `+"```"+`
`)

			// Scaffolding conditionals
			if web {
				writeFile(filepath.Join(name, "web", "package.json"), `{
  "name": "web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "@tanstack/react-query": "^5.0.0",
    "astra-client": "file:../shared/astra-client"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.2.1",
    "typescript": "^5.2.2",
    "vite": "^5.1.4"
  }
}`)
			}

			if mobile {
				writeFile(filepath.Join(name, "mobile", "package.json"), `{
  "name": "mobile",
  "version": "1.0.0",
  "main": "node_modules/expo/AppEntry.js",
  "scripts": {
    "start": "expo start",
    "android": "expo start --android",
    "ios": "expo start --ios"
  },
  "dependencies": {
    "expo": "~50.0.0",
    "expo-secure-store": "~12.8.1",
    "react": "18.2.0",
    "react-native": "0.73.4",
    "astra-client": "file:../shared/astra-client"
  }
}`)
				writeFile(filepath.Join(name, "mobile", "lib", "auth.ts"), `
import * as SecureStore from 'expo-secure-store';

export const tokenStorage = {
  get: () => SecureStore.getItemAsync('token'),
  set: (val: string) => SecureStore.setItemAsync('token', val),
  remove: () => SecureStore.deleteItemAsync('token')
};
`)
			}

			fmt.Println("Astra application created successfully!")
		},
	}

	cmd.Flags().BoolVar(&apiOnly, "api-only", false, "Create an API only application")
	cmd.Flags().BoolVar(&web, "web", false, "Create a web frontend with React + Vite")
	cmd.Flags().BoolVar(&mobile, "mobile", false, "Create a mobile frontend with Expo")

	return cmd
}
