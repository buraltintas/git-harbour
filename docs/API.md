# HTTP API

Private JSON routes require `Authorization: Bearer <GitHarbour application token>`.

## Authentication

- `GET /auth/github/start` creates a hashed 10-minute state and redirects to GitHub with no explicit scope.
- `GET /auth/github/callback` consumes state, exchanges the GitHub code server-side, imports public data, and redirects with a 90-second exchange code.
- `POST /auth/exchange {code}` consumes the code and returns `{token,user}`.
- `POST /auth/logout` revokes the current session.
- `POST /v1/dev/session` exists only when `APP_ENV` is development/test and `GITHARBOUR_DEV_AUTH=true`.

## Private user and Solo

- `GET /v1/me`
- `GET /v1/me/contributions`
- `POST /v1/games/solo {startDate,fleet}`
- `GET /v1/games/{id}`
- `POST /v1/games/{id}/shots {x,y}`

Game reads/shots are scoped to the authenticated owner. Requests submit only target intent; hit, AI turn, winner, stats and share are derived server-side.

## Challenge PvP

- `POST /v1/challenges` creates a seven-day private link.
- `GET /v1/public/challenges/{code}` returns its safe state and, when authenticated, allowed actions.
- `POST /v1/challenges/{code}/accept` atomically claims an open challenge.
- `POST /v1/challenges/{code}/cancel` lets its creator cancel while open.
- `POST /v1/challenges/{code}/ready {startDate,fleet}` freezes one player's setup once.
- `GET /v1/battles` groups active turns, waits, and completed battles.
- `GET /v1/games/{id}` and `POST /v1/games/{id}/shots {x,y}` dispatch to Solo or PvP without trusting client outcomes.
- `POST /v1/games/{id}/rematch` creates one idempotent, prior-opponent-only challenge.

## Public

- `GET /v1/public/users/{login}` returns the safe public projection.
- `GET /v1/public/leaderboards/pvp` returns ranked players with completed PvP games.
- `GET /u/{login}` returns canonical escaped HTML and real 404 HTML for unknown users.
- `GET /widgets/{login}.svg?theme=light|dark` returns a 480×140 embeddable SVG.
- `GET /share/users/{login}.svg` remains an alias for widget compatibility.
- `GET /s/{shareId}` returns completed-game Open Graph/Twitter HTML.
- `GET /share/games/{shareId}.png` returns a 1200×630 result card for completed PvP shares.
- `GET /healthz`

Public DTOs never include GitHub tokens, session/auth fields, email, database identifiers, hidden fleets, or unrevealed enemy dates. Errors use `{error:{code,message}}`.

Mutation conflicts use stable codes including `challenge_taken`, `self_challenge`, `setup_locked`, `not_your_turn`, `duplicate_shot`, and `game_complete`. Unknown internal errors are not reflected to clients.
