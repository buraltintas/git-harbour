# HTTP API

All JSON endpoints are under `/v1`. Production requests use `Authorization: Bearer <GitHarbour token>`. Development mode supplies a local session through `/v1/dev/session`.

## Session and contributions

- `GET /auth/github/start` begins OAuth.
- `GET /auth/github/callback` completes GitHub exchange server-side and redirects with a one-time code.
- `POST /auth/exchange {code}` returns `{token, user}` and consumes the code.
- `POST /v1/dev/session` returns a local token and mock user when enabled.
- `GET /v1/me` returns identity, separate Solo/PvP stats, and rank.
- `GET /v1/me/contributions` returns normalized days grouped into 52 aligned weeks.

## Solo games

- `POST /v1/games/solo {startDate, fleet:[{kind,cells:[{x,y}]}]}` snapshots the selected board, securely chooses another period and enemy fleet, and returns a public game.
- `GET /v1/games/{id}` returns the persisted public projection.
- `POST /v1/games/{id}/shots {x,y}` locks the game, validates the player's turn and unique coordinate, resolves the shot, executes one AI turn if needed, and returns the updated projection plus shot events.

Before completion the projection includes both contribution patterns but not enemy dates or fleet coordinates. After completion it adds `enemyPeriod`, result, rating delta, updated Solo stats, and `shareId`.

## PvP-ready contracts

- `POST /v1/challenges` (future) creates a challenge after the creator's snapshot and fleet are validated.
- `POST /v1/challenges/{code}/join` (future) accepts the opponent snapshot and fleet.
- The same game/shot resource supports `mode=pvp`, two user player rows, alternating locked turns, and independent PvP stats.

## Sharing and support

- `GET /s/{shareId}` returns server-generated HTML with `og:title`, `og:description`, `og:image`, and `twitter:card=summary_large_image`.
- `GET /share/games/{shareId}.png` returns a replaceable 1200×630 result renderer.
- `GET /share/users/{login}.svg` returns a replaceable README stats renderer.
- `GET /healthz` reports service health.

Errors use `{error:{code,message}}` with appropriate 4xx/5xx status. Clients never submit hit/miss, winner, rating delta, stats delta, enemy range, or AI actions.
