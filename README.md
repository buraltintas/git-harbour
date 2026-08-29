# GitHarbour

**Your GitHub history is a battlefield.** GitHarbour turns a frozen 10-week slice of a GitHub contribution calendar into a fair, server-authoritative Battleship-style game.

This repository contains the first complete Solo vertical slice and the persistence/auth/domain seams for challenge-link PvP. Contribution activity affects board appearance only.

## Run locally

Requirements: Node 20+, Go 1.25+, Docker, and Docker Compose.

```bash
cp .env.example .env
docker compose up -d postgres
set -a; source .env; set +a
(cd apps/api && go run ./cmd/api)
npm install
npm run dev:web
```

Open [http://localhost:5173](http://localhost:5173). Mock development auth and a deterministic 52-week public contribution calendar work without GitHub credentials. Games persist across refreshes when `DATABASE_URL` is set and PostgreSQL is running. The API also runs without a database using in-memory state for quick UI work.

Run every test with `npm test`; build the production Pages bundle with `npm run build:web`.

## Configuration

Copy `.env.example`. The API uses `DATABASE_URL`, `PORT`, `WEB_ORIGIN`, and `PUBLIC_API_URL`. Production OAuth additionally requires `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_OAUTH_CALLBACK`, and `GITHUB_WEB_CALLBACK`. The web build uses `VITE_API_URL` and `VITE_BASE_PATH`; the Pages workflow sets the repository sub-path automatically.

GitHub OAuth is intentionally API-mediated: the secret and GitHub token stay server-side; the Pages client receives a short-lived one-time exchange code and trades it for a GitHarbour bearer token. The current vertical slice exposes the start endpoint and documented exchange contract; callback/token persistence is the next production-integration step.

## Structure

- `apps/web` — React, TypeScript, Vite, Primer, TanStack Query, hash-router-ready frontend
- `apps/api` — Go API, game/AI/rating domain, PostgreSQL integration, migrations, share metadata/SVG endpoints
- `docs` — product, game rules, architecture, UI, and API sources of truth
- `.github/workflows` — Pages build/deploy and API CI/container verification

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for security and persistence boundaries.
