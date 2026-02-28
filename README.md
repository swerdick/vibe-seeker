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
| Backend | Go (`net/http`, stdlib-first), SQLite (`modernc.org/sqlite`) |
| Frontend | React 19, Vite, TypeScript, TanStack Query, React Map GL, Tailwind CSS |
| Auth | Spotify OAuth2 → self-issued HMAC-SHA256 JWTs |
| Hosting | AWS CloudFront + S3 (frontend) + App Runner (backend) |
| Observability | OpenTelemetry → Grafana Cloud |
| CI/CD | GitHub Actions |

## Prerequisites

- [Go 1.23+](https://go.dev/dl/)
- [Node.js 22+](https://nodejs.org/)
- [Podman](https://podman.io/)

## Getting Started

### Backend

```bash
cd backend
go run ./cmd/server/
# http://localhost:8080/api/health
```

See [backend/README.md](backend/README.md) for container build and run instructions.

### Frontend

```bash
cd frontend
npm install
npm run dev
# http://localhost:5173
```

See [frontend/README.md](frontend/README.md) for container build and run instructions.

## Project Structure

```
vibe-seeker/
├── backend/          # Go API server
├── frontend/         # React SPA
└── README.md
```

## Design Docs

Architecture, API design, data model, matching algorithm, and other design docs live in the [project-hub](../project-hub/concert-radar/docs/) repository.
