# GitHarbour product

GitHarbour turns real public GitHub activity into strategy: **activity builds the fleet; intensity powers it.** Active-day breadth provides more deployable units, while each day's contribution intensity provides that coordinate's continuous, diminishing-return combat power. These are deliberately separate dimensions.

## Battle Your History

Solo is Player vs Computer using two frozen ten-week periods from the authenticated developer's own real history.

The player compares periods using date range, total contributions, active days, Contribution Power, Fleet Capacity, peak day, and maximum deployable power. After locking a period, the player chooses which eligible active dates become units when candidates exceed capacity. Contribution units cannot move. Weak, explicit Reserves occupy inactive cells only when needed, so zero-contribution users can play without fabricated activity.

The server randomly freezes a different real period, preferring a non-overlapping alternative, and deploys the computer under the same capacity, power, and Reserve rules. No synthetic GitHub identity or contribution calendar exists.

Each side takes one complete action: select a surviving attacker, then select an untargeted opponent coordinate. Misses eliminate nobody. A clash compares continuous unit power through bounded probabilistic combat and eliminates exactly one participant. Attacking exposes that unit; clashing exposes the defender. Power improves odds but never guarantees victory. The first side to eliminate every deployed enemy unit wins.

The result reveals both histories and deployments and explains fleet breadth, power, actions, misses, clash record, survivors, and rating. V3 Solo W/L, clash rate, streaks, and Elo are isolated from incompatible legacy target-hunt statistics.

## Explicit non-goals

There are no Carrier/Battleship/Cruiser/Submarine/Destroyer classes, fixed 5/4/3/3/2 fleet, orientation, ship shapes, adjacency, movement, HP, or sunk-by-length behavior. Contributions are not decorative and positive days are candidates—not automatic targets.

PvP is not part of `contribution_fleet_v3`. Existing identity, challenge, rating, history, and leaderboard infrastructure stays preserved and gated until a separately approved PvP ruleset is defined.

If no distinct second ten-week period exists, Solo reports the case and does not manufacture history. The fallback remains intentionally unresolved pending product confirmation.

## Preserved platform

GitHub OAuth, application sessions, contribution import, Supabase PostgreSQL/pgx, frozen snapshot persistence, public profiles, versioned Solo/PvP stats, stable shares, canonical HTML, README widgets, GitHub Pages, Koyeb, Primer styling, mobile layouts, keyboard navigation, and authorization boundaries remain intact.
