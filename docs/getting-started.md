# Getting Started with Astra

Astra is designed to get you from zero to production as quickly as possible without sacrificing quality or security.

## Prerequisites

- [Go](https://golang.org/dl/) (1.22 or later)
- [Docker](https://www.docker.com/get-started) (for running local dependencies like Postgres/Redis)
- [Make](https://www.gnu.org/software/make/)

## 1. Install the Astra CLI

The `astra` CLI is your best friend when developing with the framework. It handles project scaffolding, code generation, and running your development server.

```bash
go install github.com/astraframework/astra/cli/astra@latest
```

## 2. Create Your First App

Let's create a new API project:

```bash
astra new hello-astra --api-only
cd hello-astra
```

The CLI will create a new directory with the canonical Astra structure.

## 3. Launch Dependencies

Astra uses Docker Compose by default for development dependencies.

```bash
make setup
```

This command will:
1. Initialize your `.env` file.
2. Start Postgres and Redis containers.
3. Run any initial database migrations.

## 4. Run the Development Server

Astra includes a built-in development server with **Hot-ReloadING**.

```bash
astra dev
```

Your server is now live at `http://localhost:3333`.

## 5. Explore the Code

Open your project in your favorite IDE (we recommend VS Code with the Go extension).

- **`routes/api.go`**: Define your API endpoints.
- **`app/controllers/`**: Place your request logic here.
- **`app/models/`**: Define your database schema using the Astra ORM.

## Next Steps

- Learn about [Routing & Middleware](routing.md)
- Explore the [ORM & Migrations](orm.md)
- Secure your app with the [Security Checklist](security-checklist.md)
