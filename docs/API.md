# HTTP API

Private JSON routes require `Authorization: Bearer <GitHarbour application token>`. The client submits intent; the server derives capacity, power, legality, randomness, eliminations, computer actions, and winner.

## Authentication and history

- `GET /auth/github/start`
- `GET /auth/github/callback`
- `POST /auth/exchange {code}`
- `POST /auth/logout`
- `POST /v1/dev/session` (development/test only)
- `GET /v1/me`
- `GET /v1/me/contributions`

## Contribution Fleet Solo

- `POST /v1/games/solo {playerStart}` freezes the selected period and a server-chosen distinct real period, prepares the hidden computer deployment, and returns `status: deployment`.
- `POST /v1/games/{id}/deployment {units:[{x,y,kind}]}` validates the exact player deployment and locks it. `kind` is `contribution` or `reserve`.
- `GET /v1/games/{id}` reads the owner-scoped state.
- `POST /v1/games/{id}/shots {target:{x,y}}` submits one player shot. Unless terminal, the same transaction also resolves one computer shot. Previously targeted coordinates are closed.

During setup/battle, `playerCells` contains the owner's dated frozen snapshot, derived power/level, deployed units, alive/exposed state, and computer target marks. `enemyCells` is allow-listed:

```json
{"x":2,"y":4,"state":"unknown"}
{"x":2,"y":5,"state":"unknown","targeted":true}
{"x":2,"y":6,"state":"exposed","combatLevel":3}
{"x":3,"y":0,"state":"eliminated","combatLevel":1,"targeted":true}
```

Before completion, enemy cells never expose dates, weekdays, contribution counts/levels, deployment, or Reserve/contribution kind. The shot event projection includes actor, coordinate, and HIT/MISS result.

Completed responses add the exact enemy period, full snapshot/deployment/powers, combat history, `winner`, `ratingDelta`, and `shareId`. `contribution_targets_v2` completed games remain readable as their own historical ruleset; active legacy games are not converted.

## Asynchronous PvP

- `GET /v1/pvp/harbour` reads the authenticated developer's current open harbour.
- `POST /v1/pvp/harbour {playerStart,units}` validates, freezes, deploys, and opens a harbour.
- `POST /v1/pvp/harbour/close` prevents new challenges without altering existing battles.
- `GET /v1/pvp/harbours` lists other real open developers without exposing dates, cells, or deployments.
- `POST /v1/pvp/games {opponentLogin}` starts a battle between two open harbours; the challenger starts.
- `GET /v1/pvp/games` lists the caller's asynchronous battles (as challenger and defender) with role-relative status, opponent, surviving units, and outcome.
- `GET /v1/pvp/games/{id}` reads the participant-scoped battle projection.
- `POST /v1/pvp/games/{id}/shots {target:{x,y}}` resolves one challenger shot and, unless terminal, one random defender return shot in the same transaction.

## Public routes

Public routes remain `/v1/public/users/{login}`, `/v1/public/leaderboards/pvp`, `/u/{login}`, `/widgets/{login}.svg`, `/s/{shareId}`, `/share/games/{shareId}.png`, and `/healthz`. The widget reads live, isolated Arcade and PvP v4 statistics and is served with revalidation/no-store headers.

Errors use `{error:{code,message}}`. `history_not_playable` explicitly represents the unresolved case where no distinct real opponent period exists.
