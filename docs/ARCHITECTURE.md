# Architecture

## Production boundary

`GitHub Pages (React/Vite/Primer) → HTTPS → Koyeb (Go container) → pgx → Supabase PostgreSQL`. The API also calls GitHub OAuth and GraphQL. Supabase is PostgreSQL only; the browser does not know it exists.

GitHub Pages cannot server-render arbitrary per-user metadata. The Pages root is a crawlable static landing page, while the API serves canonical HTML at `/u/{login}` and `/s/{shareId}`. Hash routes such as `#/u/{login}` provide interactive app views without fragile Pages rewrites.

## Identity and sessions

GitHub’s numeric ID is the durable external identity; current login casing is updated on each sign-in. OAuth state and login exchange codes are random, hashed at rest, expiring, transactionally single-use values. Application sessions are opaque 30-day bearer tokens and only their SHA-256 hashes are stored. GitHub tokens are used synchronously to import public identity/contributions and are discarded.

The Pages architecture requires storing the GitHarbour token in browser local storage. This is more exposed to XSS than an httpOnly cookie, so dynamic output is escaped, unsafe HTML is avoided, the OAuth code is removed immediately, token values are never logged, and a static meta CSP limits sources. GitHub Pages cannot provide ideal response-header CSP, so the meta policy is a mitigation rather than a complete boundary.

## Persistence and concurrency

Production requires `DATABASE_URL`; config uses `pgxpool.ParseConfig`, with conservative configurable limits. Migrations run explicitly via `cmd/migrate`, never during instance startup. Solo and PvP shot transactions lock the game before resolving intent. PvP also locks both statistics rows at completion, calculates both outcomes from pre-match ratings, writes normalized shots/results and the unique share, then commits once. Duplicate concurrent accepts and shots serialize; only one state transition wins.

The memory repository and mock calendar exist only in development/test with explicit dev auth. Production startup fails for missing/unreachable PostgreSQL or missing auth/game schema.

## Data model

- `users`: UUID identity, canonical current GitHub login/name/avatar.
- `github_identities`: durable unique numeric GitHub ID; no OAuth token column after migration 002.
- `contribution_days`: normalized public date/count/level data refreshed at login.
- `oauth_states`, `login_exchange_codes`, `auth_sessions`: hashed authentication credentials with expiry/consumption/revocation.
- `games`: mode/lifecycle, opening/current/winning player and Solo authoritative state.
- `game_players`: two immutable PvP board/fleet snapshots, selected periods, readiness and shot views.
- `pvp_shots`, `pvp_results`: normalized turns and one immutable terminal result per participant.
- `mode_stats`: independent Solo/PvP rows.
- `challenges`: challenge-link preparation only.
- `shares`: stable unique public result ID per completed game.

Contribution refresh never touches snapshots inside existing games. Hidden enemy fleet/date state exists in database JSON but is removed from browser projections until completion.

## Public and deployment surfaces

The API exposes safe public JSON, canonical HTML, escaped SVG widgets, and completed-game share HTML. CORS echoes only configured origins. Pages receives only public `VITE_API_URL`/`VITE_BASE_PATH`. Koyeb receives server configuration and secrets; Supabase and GitHub secrets never enter GitHub Actions’ web build.

The web polls only active challenge/battle views. PostgreSQL remains the sole coordination authority; no direct browser database access, Realtime channel, or WebSocket is involved.

PvP mutations also have a conservative per-session (or unauthenticated IP) process-local request limit. PostgreSQL constraints and row locks remain the integrity boundary across Koyeb instances. A shared edge/distributed limiter and notification delivery are future operational enhancements if traffic requires them.
