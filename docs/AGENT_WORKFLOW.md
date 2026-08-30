# Development workflow

GitHarbour changes are reviewed along three boundaries before integration:

1. **Game flow** owns contribution-window quality, target resolution, victory and rating transitions. It does not perform I/O. `contributionCount > 0` is always one target and no active Solo flow may accept a date range or placement payload.
2. **Backend integrity** owns migrations, authorization, locking, persistence and safe public projections.
3. **Frontend UX** owns routes, polling lifecycle, keyboard/mobile behavior and friendly error presentation.
4. **QA/product review** verifies cross-boundary contracts, production builds and real browser flows.

Keep commits focused by boundary, preserve independent Solo/PvP statistics, and never move authority for target detection, turns, snapshots or ratings into the browser. Active projections must be allow-listed; do not serialize internal snapshots and hide fields afterward. Required checks are `npm test`, `npm run build:web`, `go test ./...`, and `go vet ./...`. Destructive database integration tests run only against an isolated disposable database explicitly supplied as `GITHARBOUR_INTEGRATION_DATABASE_URL`; never point them at production.

The repository currently has no executable project-specific agent harness or checked-in skill package. These roles describe the actual review split used by contributors and coding agents; repository instructions take precedence if a future `AGENTS.md` or `.agents/` harness is added.
