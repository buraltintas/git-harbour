# HTTP API

Private JSON routes require `Authorization: Bearer <GitHarbour application token>`.

## Authentication and users

- `GET /auth/github/start`
- `GET /auth/github/callback`
- `POST /auth/exchange {code}`
- `POST /auth/logout`
- `POST /v1/dev/session` only in explicit development/test mode
- `GET /v1/me`
- `GET /v1/me/contributions`

## Battle Your History

- `POST /v1/games/solo {}` selects and freezes a hidden server-chosen window.
- `GET /v1/games/{id}` reads an owner-scoped game.
- `POST /v1/games/{id}/shots {x,y}` submits coordinate intent only.

An active response contains summary fields and exactly 70 allow-listed cell projections:

```json
{"x":2,"y":4,"state":"unknown"}
{"x":2,"y":5,"state":"miss"}
{"x":2,"y":6,"state":"hit","contributionCount":14,"contributionLevel":3}
```

Unknown and missed cells never include date, weekday, contribution count, or contribution level. The active response includes `targetCount`, `foundCount`, `shots`, `misses`, and `accuracy` so the completion objective is clear without revealing positions.

Only a completed response adds `period`, `totalContributions`, `ratingDelta`, `shareId`, and the full dated cell snapshot. A legacy prototype game returns `410 legacy_game_retired` instead of being reinterpreted under new rules.

## PvP transition

Read-only public profile, leaderboard, archived history, and existing challenge records remain available. New challenge creation, acceptance, and readiness return `503 pvp_refit` until contribution-target PvP setup is implemented. This avoids exposing an active mixed-ruleset product.

## Public

- `GET /v1/public/users/{login}`
- `GET /v1/public/leaderboards/pvp`
- `GET /u/{login}`
- `GET /widgets/{login}.svg?theme=light|dark`
- `GET /share/users/{login}.svg`
- `GET /s/{shareId}`
- `GET /share/games/{shareId}.png`
- `GET /healthz`

Public shares exist only for completed games. Errors use `{error:{code,message}}`; hidden internal state, auth values, GitHub tokens, database identifiers, and active periods are never serialized.
