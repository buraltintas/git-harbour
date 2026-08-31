# Architecture

## Production boundary

`GitHub Pages (React/Vite/Primer) → HTTPS → Koyeb (Go container) → pgx → Supabase PostgreSQL`. The API also calls GitHub OAuth and GraphQL. Supabase is PostgreSQL only; the browser does not know it exists.

GitHub Pages cannot server-render arbitrary per-user metadata. The Pages root is a crawlable static landing page, while the API serves canonical HTML at `/u/{login}` and `/s/{shareId}`. Hash routes such as `#/u/{login}` provide interactive app views without fragile Pages rewrites.

## Identity and sessions

GitHub’s numeric ID is the durable external identity; current login casing is updated on each sign-in. OAuth state and login exchange codes are random, hashed at rest, expiring, transactionally single-use values. Application sessions are opaque 30-day bearer tokens and only their SHA-256 hashes are stored. GitHub tokens are used synchronously to import public identity/contributions and are discarded.

The Pages architecture requires storing the GitHarbour token in browser local storage. This is more exposed to XSS than an httpOnly cookie, so dynamic output is escaped, unsafe HTML is avoided, the OAuth code is removed immediately, token values are never logged, and a static meta CSP limits sources. GitHub Pages cannot provide ideal response-header CSP, so the meta policy is a mitigation rather than a complete boundary.

## Persistence and concurrency

Production requires `DATABASE_URL`; config uses `pgxpool.ParseConfig`, with conservative configurable limits. Migrations run explicitly via `cmd/migrate`, never during instance startup, and startup requires migration 008. Deployment and combat transactions lock the game; combat also locks v3 Solo statistics before atomically resolving the player action and, unless terminal, one computer response. Concurrent duplicates cannot create an extra turn. Terminal stats and the unique share commit once. Both immutable snapshots, deployments, exposure/elimination flags, probabilities, and rolls live in persisted game state, so refreshed imports cannot alter a match.

The memory repository and mock calendar exist only in development/test with explicit dev auth. Production startup fails for missing/unreachable PostgreSQL or missing auth/game schema.

## Data model

- `users`: UUID identity, canonical current GitHub login/name/avatar.
- `github_identities`: durable unique numeric GitHub ID; no OAuth token column after migration 002.
- `contribution_days`: normalized public date/count/level data refreshed at login.
- `oauth_states`, `login_exchange_codes`, `auth_sessions`: hashed authentication credentials with expiry/consumption/revocation.
- `games`: mode/lifecycle, explicit `fleet_v1`, `contribution_targets_v2`, or active `contribution_fleet_v3` ruleset, and authoritative frozen Solo state.
- `game_players`: two immutable future PvP contribution snapshots, selected periods and readiness; the legacy fleet column remains only for archived prototype rows.
- `pvp_shots`, `pvp_results`: archived normalized prototype records plus additive contribution reveal/target-stat fields for the next PvP phase.
- `mode_stats`: preserved pre-pivot Solo counters and current PvP statistics.
- `legacy_mode_stats`: immutable copies of pre-pivot fleet counters and temporary one-sided v2 history-hunt counters.
- `ruleset_mode_stats`: isolated per-ruleset Solo W/L, action/clash rate, streak, and Elo counters. V2 and v3 are never mixed.
- `challenges`: challenge-link preparation only.
- `shares`: stable unique public result ID per completed game.

Contribution refresh never touches snapshots inside existing games. The owner sees their own dates, derived powers, and deployment. Active Enemy Harbour responses are constructed from an allow-list: unknown cells reveal coordinates only; exposure adds combat level but not date, count, exact power, or unit kind. The full enemy period/snapshot/deployment is projected only after completion. Historical rulesets remain versioned and are never silently reinterpreted.

## Public and deployment surfaces

The API exposes safe public JSON, canonical HTML, escaped SVG widgets, and completed-game share HTML. CORS echoes only configured origins. Pages receives only public `VITE_API_URL`/`VITE_BASE_PATH`. Koyeb receives server configuration and secrets; Supabase and GitHub secrets never enter GitHub Actions’ web build.

The web polls only active challenge/battle views. PostgreSQL remains the sole coordination authority; no direct browser database access, Realtime channel, or WebSocket is involved.

PvP mutations also have a conservative per-session (or unauthenticated IP) process-local request limit. PostgreSQL constraints and row locks remain the integrity boundary across Koyeb instances. A shared edge/distributed limiter and notification delivery are future operational enhancements if traffic requires them.
