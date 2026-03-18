# Vibe Seeker

A venue-centric discovery app for New York City. Connect your Spotify account, and Vibe Seeker builds a taste profile from your listening history, then scores nearby bars, clubs, and small venues based on the genres and artists they typically book.

Find places where you'll love the music, it won't cost $100, and you might discover your next favorite spot.

## How It Works

1. Log in with Spotify
2. The app reads your top artists and genres
3. NYC venue data is fetched from SeatGeek, Oh My Rockness, and Ticketmaster
4. Each venue gets a taste profile built from its booking history
5. Venues are scored against your taste using cosine similarity
6. An interactive map shows scored venues with match reasons and upcoming shows

## Stack

| Layer | Tech |
|-------|------|
| Backend | Go (`net/http`, stdlib-first), PostgreSQL |
| Frontend | React 19, Vite, TypeScript, TanStack Query, React Map GL, Tailwind CSS |
| Auth | Spotify OAuth2 → self-issued HMAC-SHA256 JWTs (HttpOnly cookies) |
| Hosting | AWS CloudFront + S3 (frontend) + App Runner (backend) |
| Observability | OpenTelemetry → Grafana Cloud |
| CI/CD | GitHub Actions |

## Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- [Node.js 22+](https://nodejs.org/)
- [Podman](https://podman.io/)
- [just](https://github.com/casey/just) (command runner)

## Getting Started

### Quick Start (Containers)

```bash
cp backend/.env.example backend/.env  # then fill in Spotify credentials and JWT_SECRET
just dev                               # starts postgres, backend, and frontend
# http://127.0.0.1:5173
```

> **Note:** Use `127.0.0.1` instead of `localhost` — the Spotify API forbids `localhost` in redirect URIs.

### Manual (No Containers)

```bash
# Backend
cd backend
go run .
# http://127.0.0.1:8080/api/health

# Frontend (separate terminal)
cd frontend
npm install
npm run dev
# http://127.0.0.1:5173
```

See [backend/README.md](backend/README.md) and [frontend/README.md](frontend/README.md) for more details.

## Project Structure

```
vibe-seeker/
├── backend/          # Go API server
├── frontend/         # React SPA
├── compose.yml       # Local dev (postgres, backend, frontend)
├── justfile          # Task runner recipes
└── README.md
```

## Useful Commands

```bash
just dev              # Start all services for local development
just ci               # Run full CI pipeline (lint, test, build, container build)
just test             # Run unit tests (backend + frontend)
just check            # Run static analysis (backend + frontend)
just fmt              # Format code (backend + frontend)
just up               # Start all services in production containers
just down             # Stop all services
```


## License

Licensed under the [Apache License 2.0](LICENSE).
