# Vibe Seeker — Frontend

React 19 + TypeScript + Vite. Serves the single-page app for the Vibe Seeker NYC venue discovery map.

## Prerequisites

- [Node.js 22+](https://nodejs.org/)
- [just](https://github.com/casey/just) (task runner)
- [Podman](https://podman.io/) (for container builds)

## Development

```bash
npm install       # Install dependencies (first time)
just dev          # Start dev server with HMR
just test         # Run unit tests (vitest)
just check        # TypeScript type-check + ESLint
just fmt          # Format code with Prettier
```

The dev server starts on http://127.0.0.1:5173 with hot module replacement.

## Build

```bash
just build            # Production build (output in dist/)
just container-build  # Build the container image
```

## CI

```bash
just ci    # Full pipeline: check, test, test-integration, build, container-build
just all   # fmt + ci
```

## Project Structure

```
frontend/
├── src/
│   ├── App.tsx               # Root component with routing
│   ├── main.tsx              # Entrypoint
│   ├── pages/                # Page components (Login, Callback, Home)
│   └── assets/               # Static assets
├── public/
├── index.html
├── Containerfile             # Multi-stage container build (node → nginx)
├── nginx.conf                # SPA-aware nginx config
├── justfile                  # Task runner recipes
├── package.json
├── tsconfig.json
├── vite.config.ts
└── README.md
```
