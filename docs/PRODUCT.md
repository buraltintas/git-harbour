# GitHarbour product

GitHarbour’s core object is a real public GitHub contribution calendar: **“Your GitHub history is a battlefield.”** The contribution cells themselves are the targets. There is no separate placement layer and contribution intensity never adds health, power, damage, probability, or extra turns.

## Battle Your History

Solo is a focused history-discovery game. The server selects a playable, contiguous ten-week slice from the authenticated developer’s imported history and immediately returns a concealed 10×7 board. A day with one or more contributions is one target; a day with no contributions is empty water. The harbour clears when every target is found.

The exact dates and untouched contribution pattern remain hidden during play. A hit may reveal that cell’s frozen contribution count and level. Completion reveals the full dated snapshot, its place in the developer’s history, shots, misses, accuracy, total contributions, rating change, and a stable share.

Solo rating is density-aware and deterministic. It compares shots used with the expected position of the final target in a random ordering, divides the difference by three, and clamps the change to −12…+12. This rewards efficient discovery without inherently punishing sparse histories. Existing public `wins` represent completed Solo history hunts.

## Developer vs Developer

The existing challenge, participant, turn, Elo, history, and leaderboard infrastructure is preserved. New PvP creation is temporarily gated while its setup and battle contracts move to the same contribution-target model: each player will freeze an eligible ten-week slice, take one shot per turn, and win by finding every target on the opponent’s harbour. Prototype fleet records remain archived, never reinterpreted.

## Preserved platform

GitHub OAuth, application sessions, contribution import, Supabase PostgreSQL/pgx, public profiles, separate Solo/PvP stats, stable share IDs, canonical HTML, README widgets, social cards, GitHub Pages, Koyeb deployment, Primer styling, and authorization boundaries remain intact.

The separate prototype fleet mechanic was replaced before public launch. No new game accepts ship coordinates or a client-selected Solo date range.
