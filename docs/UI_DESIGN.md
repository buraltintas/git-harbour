# UI design

GitHarbour uses Primer controls/tokens and a restrained GitHub-adjacent visual language. Contribution grids and harbour terminology provide playfulness; there is no glass, neon, decorative motion, custom cursor, or oversized spectacle.

## Surfaces

- The unauthenticated static landing explains sign-in, harbour selection, Solo play, ratings, public profiles and README widgets. Production offers only **Continue with GitHub**.
- The authenticated dashboard preserves the four-stage Solo flow and adds public-profile discovery/copy actions plus logout.
- `#/challenge/new`, `#/challenge/{code}`, `#/battles`, and `#/battle/{id}` cover creation, acceptance, immutable setup, waiting, turns, results and rematches.
- `#/leaderboard` and public profile histories expose only completed PvP outcomes.
- The interactive `#/u/{login}` page shows safe public identity, independent Solo/PvP stats, member date, contribution preview, widget theme preview and copyable Markdown.
- API `/u/{login}` is useful visible HTML as well as metadata; it is intentionally simpler and canonical for crawlers/social previews.

All boards retain keyboard focus, descriptive cell labels, non-color hit/miss cues, live turn status, disabled invalid actions, reduced-motion support and horizontal overflow rather than undersized mobile targets. Desktop and approximately 390px layouts are quality gates. Active PvP screens poll conservatively and stop when unmounted or complete. Dynamic HTML/SVG content is escaped.

The meta CSP permits the configured API through HTTPS and GitHub-hosted avatars; because it is a Pages-compatible meta tag and Primer needs inline styling, it is a practical mitigation rather than an ideal header policy.
