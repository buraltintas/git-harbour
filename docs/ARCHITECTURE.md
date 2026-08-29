# Architecture

## Repository

`apps/web` is a React/Vite/TypeScript GitHub Pages application using Primer, React Router's hash router, and TanStack Query. `apps/api` is an idiomatic Go HTTP service using chi and pgx. PostgreSQL is the durable store, started locally with Docker Compose. SQL migrations live in `apps/api/migrations`.

## Trust boundary

The API is authoritative. Domain packages own fleet validation, shots, AI targeting, Elo, and statistics. HTTP handlers translate requests and return a public game projection. Persistence stores both public and hidden state, but opponent ship coordinates and the enemy date range are omitted until allowed. PostgreSQL row locks and a terminal-update marker serialize turns and make stats updates idempotent.

For frictionless development, `GITHARBOUR_DEV_AUTH=true` exposes a deterministic mock user and 52-week contribution calendar. Production contribution import uses GitHub GraphQL `contributionCalendar`; HTML is never scraped.

## Authentication

Production uses GitHub OAuth with public identity only and no repository scope. The Pages client opens `GET /auth/github/start`; the API stores OAuth state, handles GitHub's callback, exchanges the GitHub code server-side, imports public contributions, creates a short-lived single-use exchange code, and redirects to the configured web callback. The client immediately posts that code to `/auth/exchange`, receives a GitHarbour bearer token, and removes the code from the URL. GitHub tokens never reach the browser and core sessions do not rely on cross-site cookies.

## Data model

- `users`, `github_identities`, and `contribution_days` hold identities and normalized public data.
- `games` represents solo or PvP lifecycle, current turn, winner, frozen board JSON, secret enemy start, and terminal update state.
- `game_players` supports one solo player plus AI now and two users later, with separate fleet JSON and shot history.
- `challenges` reserves challenge-link state for PvP.
- `mode_stats` has one row per user/mode; Solo and PvP ratings never mix.
- `shares` maps stable public IDs to completed games.

Snapshots are embedded in a game and immutable after creation. Pooled transactions lock the game row before shots. Database constraints enforce modes/statuses and unique shot coordinates; the domain enforces geometry and turn order.

## Deployment

The web build receives `VITE_API_URL` and `VITE_BASE_PATH`, and is deployed to GitHub Pages by workflow. The API builds into a minimal Cloud Run-compatible container, listens on `PORT`, and uses `DATABASE_URL`. CORS is restricted by `WEB_ORIGIN`.

