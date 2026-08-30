# GitHarbour

**Your GitHub history is a battlefield.** GitHarbour turns a frozen 10-week slice of a developer’s public GitHub contribution calendar into a fair, server-authoritative strategy game.

The production shape is deliberately simple: a React/Vite/Primer application on GitHub Pages calls a Go API on Koyeb, and the API alone accesses Supabase PostgreSQL plus GitHub OAuth/GraphQL. Supabase is managed PostgreSQL only—there is no Supabase Auth, browser SDK, Realtime, Data API, or direct database access.

## Local development

Requirements: Node 20+, Go 1.25+, Docker, and Docker Compose.

```bash
cp .env.example .env
docker compose up --build
npm ci
npm run dev:web
```

Open `http://localhost:5173`. Vite development mode exposes Alice and Bob test accounts so a challenge can be exercised in one browser; production builds expose only **Continue with GitHub**. Compose runs versioned migrations once before starting the API. Without Docker, set `APP_ENV=development` and `GITHARBOUR_DEV_AUTH=true`; the API may use its development-only memory repository when `DATABASE_URL` is empty.

```bash
npm test
npm run build:web
cd apps/api && go vet ./...
```

## Production migrations and Supabase setup

1. Create a Supabase project.
2. Open **Project Settings → Database → Connection string**.
3. Use the PostgreSQL Session Pooler connection string for Koyeb's IPv4 network; retain its required `sslmode` settings and percent-encode the password.
4. Store the connection string only as a Koyeb secret named `DATABASE_URL`.
5. Run `DATABASE_URL='…' go run ./cmd/migrate up` from `apps/api`, or run the built container once with command `/migrate up`.
6. Verify `schema_migrations` contains migrations through `004_pvp.sql`.
7. Do not configure Supabase Auth.
8. Do not expose an anon key, service-role key, connection string, or any Supabase credential to the web build.

Koyeb instances never run migrations during startup. In `APP_ENV=production`, a missing/unreachable database or missing schema fails startup; there is no memory fallback. Pool defaults are five maximum connections, zero minimum connections, and a 30-minute maximum connection lifetime.

## GitHub OAuth App setup

Create an OAuth App under GitHub **Settings → Developer settings → OAuth Apps**.

Local:

- Homepage URL: `http://localhost:5173`
- Authorization callback URL: `http://localhost:8080/auth/github/callback`

Production:

- Homepage URL: `https://buraltintas.github.io/git-harbour/`
- Authorization callback URL: `https://<KOYEB_DOMAIN>/auth/github/callback`

The Pages app sends the browser to the API. The API validates a single-use state, exchanges the GitHub code server-side, imports the public identity and `contributionCalendar`, discards the GitHub token, and returns through `GITHUB_WEB_CALLBACK` with a 90-second single-use exchange code. The browser exchanges that for an opaque GitHarbour bearer token. No explicit OAuth scope is requested: GitHub documents `(no scope)` as read-only public information, which is sufficient for this public-data MVP.

## Environment placement

GitHub repository variables used by Pages:

- `API_URL`: public Koyeb base URL. It becomes `VITE_API_URL`.

GitHub repository secrets: none are required by the static Pages build. Never create a `VITE_*` secret.

Koyeb environment variables:

- `APP_ENV=production`
- `DB_MAX_CONNS=5`, `DB_MIN_CONNS=0`, `DB_MAX_CONN_LIFETIME=30m`
- `WEB_ORIGIN=https://buraltintas.github.io`
- `PUBLIC_API_URL=https://<KOYEB_DOMAIN>`
- `WEB_APP_URL=https://buraltintas.github.io/git-harbour/#/`
- `GITHUB_CLIENT_ID`
- `GITHUB_OAUTH_CALLBACK=https://<KOYEB_DOMAIN>/auth/github/callback`
- `GITHUB_WEB_CALLBACK=https://buraltintas.github.io/git-harbour/#/auth/callback`

Koyeb secrets:

- `DATABASE_URL`
- `GITHUB_CLIENT_SECRET`

GitHub Pages settings:

- Source: **GitHub Actions**
- Workflow: `.github/workflows/pages.yml`
- `VITE_BASE_PATH` is set to `/git-harbour/` from the repository name.

## Add GitHarbour to your GitHub profile

Replace `API_URL` and the login with deployed values:

```markdown
[![GitHarbour Stats](https://API_URL/widgets/octocat.svg)](https://API_URL/u/octocat)
[![GitHarbour Stats](https://API_URL/widgets/octocat.svg?theme=dark)](https://API_URL/u/octocat)
[![GitHarbour Stats](https://API_URL/widgets/octocat.svg?theme=light)](https://API_URL/u/octocat)
```

## Security notes

The static Pages/Koyeb split prevents a same-origin httpOnly session cookie. The web app therefore stores only the opaque GitHarbour application token in local storage. This creates an honest XSS trade-off: the app uses a restrictive static meta CSP, avoids unsafe HTML and third-party scripts, removes login exchange codes immediately, and never logs tokens. GitHub access tokens and all database credentials remain server-side.

## Developer vs Developer

Choose **Challenge a developer**, copy the private link, and send it to one person. Each player independently freezes a 10-week harbour and fleet. Once both are ready, the server chooses the opening player; turns can be taken asynchronously from **Battles**. Completion updates only PvP Elo/statistics, publishes safe history/share surfaces, and offers an opponent-restricted rematch link.

See [docs/KOYEB.md](docs/KOYEB.md), [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/API.md](docs/API.md), and [docs/GAME_RULES.md](docs/GAME_RULES.md).
