# Vibe Seeker — Backend

Go HTTP server using `net/http` (Go 1.22+ pattern routing). Serves the REST API on port `8080`.

## Prerequisites

- [Go 1.23+](https://go.dev/dl/)
- [Podman](https://podman.io/) (for container builds)

## Run Locally

```bash
go run .
```

The server starts on http://localhost:8080. Verify with:

```bash
curl http://localhost:8080/api/health
```

## Build Container

```bash
podman build -t vibe-seeker-api -f Containerfile .
```

## Run with Podman

```bash
podman run -p 8080:8080 vibe-seeker-api
```

## Project Structure

```
backend/
├── main.go                   # Entrypoint
├── internal/
│   ├── configuration/        # Environment / config loading
│   ├── handlers/             # HTTP handlers
│   ├── middleware/            # HTTP middleware
│   ├── observability/        # OpenTelemetry setup
│   └── webserver/            # Server setup and routing
├── Containerfile             # Multi-stage container build
├── go.mod
└── README.md
```
