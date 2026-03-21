# Vibe Seeker — Backend

Go HTTP server using `net/http` (Go 1.22+ pattern routing). Serves the REST API on port `8080`.

## Prerequisites

- [Go 1.23+](https://go.dev/dl/)
- [just](https://github.com/casey/just) (task runner)
- [Podman](https://podman.io/) (for container builds)

## Configuration

Copy the example env file and fill in your credentials:

```bash
cp .env.example .env
```

See `.env.example` for required and optional variables (Spotify credentials, JWT secret, database URL, etc.). The justfile auto-loads `.env` via `set dotenv-load`.

## Development

```bash
just dev          # Run the server locally
just test         # Run unit tests
just test-integration  # Run integration tests
just check        # Run vet + golangci-lint
just fmt          # Format code (gofmt + goimports)
```

## Build

```bash
just build            # Build the Go binary
just container-build  # Build the container image
```

## CI

```bash
just ci    # Full pipeline: check, test, test-integration, build, container-build
just all   # fmt + ci
```

## Project Structure

```
backend/
├── main.go                   # Entrypoint
├── internal/
│   ├── auth/                 # Spotify OAuth client, JWT creation/parsing
│   ├── configuration/        # Environment / config loading
│   ├── handlers/             # HTTP handlers
│   ├── middleware/            # HTTP middleware (CORS, auth)
│   ├── observability/        # OpenTelemetry setup
│   ├── store/                # Database connection, migrations, and data access
│   └── webserver/            # Server setup and routing
├── compose.yml               # Docker Compose for local Postgres
├── Containerfile             # Multi-stage container build
├── .env.example              # Example environment variables
├── justfile                  # Task runner recipes
├── go.mod
└── README.md
```
