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

- `POST /v1/games/solo {playerStart}` validates and freezes the selected Player Harbour, then securely matches a different AI Harbour.
- `GET /v1/games/{id}` reads an owner-scoped game.
- `POST /v1/games/{id}/shots {x,y}` submits coordinate intent only.

An active response contains `playerCells` and `enemyCells`, each with exactly 70 allow-listed projections. The owner receives their fully dated Player Harbour with AI shot marks. The Enemy Harbour uses:

```json
{"x":2,"y":4,"state":"unknown"}
{"x":2,"y":5,"state":"miss"}
{"x":2,"y":6,"state":"hit","contributionCount":14,"contributionLevel":3}
```

Unknown and missed enemy cells never include date, weekday, contribution count, or contribution level. Active responses include `currentTurn`, both target counts, both hit counts, player `shots`/`misses`/`accuracy`, and separate `aiShots`/`aiMisses`/`aiAccuracy`. A shot response contains one player event and, unless the player wins immediately, exactly one AI response event.

Only a completed response adds `enemyPeriod`, `ratingDelta`, `shareId`, `winner: player|ai`, and the full dated enemy snapshot. The selected `playerPeriod` is owner-visible throughout. Legacy fleet games and temporary one-sided v2 games return `410 legacy_game_retired` instead of being reinterpreted.

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
