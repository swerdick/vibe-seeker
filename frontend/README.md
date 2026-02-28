# Vibe Seeker — Frontend

React 19 + TypeScript + Vite. Serves the single-page app for the Vibe Seeker NYC venue discovery map.

## Prerequisites

- [Node.js 22+](https://nodejs.org/)
- [Podman](https://podman.io/) (for container builds)

## Run Locally

```bash
npm install
npm run dev
```

The dev server starts on http://localhost:5173 with hot module replacement.

## Lint & Type Check

```bash
npm run lint
npx tsc --noEmit
```

## Build for Production

```bash
npm run build
```

Output goes to `dist/`.

## Build Container

```bash
podman build -t vibe-seeker-frontend -f Containerfile .
```

## Run with Podman

```bash
podman run -p 3000:3000 vibe-seeker-frontend
```

The app is served at http://localhost:3000 via nginx with SPA routing.

## Project Structure

```
frontend/
├── src/
│   ├── App.tsx               # Root component
│   └── main.tsx              # Entrypoint
├── public/
├── index.html
├── Containerfile             # Multi-stage container build (node → nginx)
├── nginx.conf                # SPA-aware nginx config
├── package.json
├── tsconfig.json
├── vite.config.ts
└── README.md
```
