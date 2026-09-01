# GitHarbour product

GitHarbour turns real public GitHub activity into familiar Battleship strategy. Active-day breadth provides more deployable units within a bounded range; contribution counts remain truthful history but do not modify shot results or unit durability.

## Battle Your History

Solo is Player vs Computer using two frozen ten-week periods from the authenticated developer's own real history.

The player compares periods using date range, total contributions, active days, bounded Fleet Capacity, and peak day. After locking a period, the player chooses which eligible active dates become units when candidates exceed capacity. Contribution units cannot move. Weak, explicit Reserves occupy inactive cells only when needed, so zero-contribution users can play without fabricated activity.

The server randomly freezes a different real period, preferring a non-overlapping alternative, and deploys the computer under the same capacity and Reserve rules. No synthetic GitHub identity or contribution calendar exists.

Each side fires at one untargeted opponent coordinate. A deployed unit is a direct HIT and is eliminated; empty water is a MISS. The player never selects one of their own units as an attacker. The first side to eliminate every deployed enemy unit wins.

The result reveals both histories and deployments and explains fleet size, hits, misses, survivors, and rating. V4 Solo W/L, accuracy, streaks, and Elo are isolated from incompatible legacy rulesets.

## Explicit non-goals

There are no Carrier/Battleship/Cruiser/Submarine/Destroyer classes, fixed 5/4/3/3/2 fleet, orientation, ship shapes, adjacency, movement, HP, or sunk-by-length behavior. Contributions are not decorative and positive days are candidates—not automatic targets.

PvP is not part of `contribution_battleship_v4`. Existing identity, challenge, rating, history, and archived leaderboard infrastructure stays preserved and gated until a separately approved PvP ruleset is defined. The active leaderboard is Solo v4.

If no distinct second ten-week period exists, Solo reports the case and does not manufacture history. The fallback remains intentionally unresolved pending product confirmation.

## Preserved platform

GitHub OAuth, application sessions, contribution import, Supabase PostgreSQL/pgx, frozen snapshot persistence, public profiles, versioned Solo/PvP stats, stable shares, canonical HTML, README widgets, GitHub Pages, Koyeb, Primer styling, mobile layouts, keyboard navigation, and authorization boundaries remain intact.
