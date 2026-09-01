# GitHarbour rules

`contribution_battleship_v4` is the active Arcade ruleset. It is familiar Battleship on two frozen GitHub contribution periods: each side deploys units, then fires at one opponent coordinate per turn. The player never selects one of their own units to attack.

## Battlefield and fleet size

A harbour is a frozen, Sunday-aligned, contiguous `10 weeks × 7 weekdays = 70` real-calendar snapshot. Each cell retains its date, weekday, contribution count, and GitHub contribution level. The player inspects valid ten-week windows and locks one as the defensive board.

Fleet size uses active-day breadth:

```text
capacity = clamp(round(3 + sqrt(12 × activeDays / 7)), 3, 14)
```

This maps `0→3`, `1→4`, `10→7`, `30→10`, `50→12`, and `70→14`. Contribution count and level remain truthful history metadata but do not alter HIT/MISS or damage.

## Deployment

The player deploys exactly Fleet Capacity single-cell units before battle:

- If active days exceed capacity, the player chooses exactly `capacity` contribution-backed cells.
- If active days are at or below capacity, every active cell is required and empty slots are filled with Reserves.
- A contribution unit stays on its real date coordinate.
- A Reserve uses an inactive real cell and never invents contribution activity.
- Zero-contribution periods receive three Reserve units and remain playable.
- Units have no named classes, shapes, orientation, adjacency, movement, HP, or multi-cell sunk logic.
- Confirmation makes deployment immutable.

The computer follows the same capacity and Reserve rules and randomly chooses a legal deployment server-side.

## Solo opponent period

The computer uses another valid ten-week period from the authenticated user's same imported history. The server chooses and freezes it at creation, preferring non-overlapping alternatives. Its exact period, contribution cells, and deployment remain hidden until completion. If no distinct valid period exists, creation returns `history_not_playable`; no synthetic history is invented.

## Combat

The player begins and clicks exactly one untargeted coordinate in Enemy Harbour:

- A deployed enemy unit at that coordinate is `HIT` and is immediately eliminated.
- An empty coordinate is `MISS`; no unit is eliminated.
- Contribution count, contribution level, legacy power, and the player's own units do not participate in shot resolution.
- A previously targeted coordinate cannot be targeted again.

Unless the player's HIT completes the battle, the computer fires once at an untargeted coordinate on the player's board. It chooses using only previous shot coordinates, not the hidden deployment. The turn then returns to the player. HIT never grants an extra shot.

## Victory, reveal, and statistics

The first side to eliminate every deployed opponent unit wins. Completion reveals both frozen periods, contribution metadata, deployments, all HIT/MISS results, fleet sizes, survivors, and rating change. Further shots are rejected.

V4 Arcade statistics are isolated from earlier rulesets. `shots` is the number of player shots, `hits` is the number of deployed enemy units hit, and accuracy is `hits / shots`. W/L, streaks, average shots per win, and 32-point Elo against the fixed 1200 computer update exactly once. The active Arcade leaderboard uses only v4 statistics.

## Versioning and PvP

Completed `fleet_v1`, `contribution_targets_v2`, and `contribution_fleet_v3` records remain historical and are never reinterpreted as v4. Legacy power/clash fields may remain in storage solely for compatibility. PvP is not implemented for v4; archived PvP records and infrastructure remain preserved and do not feed the active Arcade leaderboard.

## Contributor invariant

**NEVER INFER UNSPECIFIED GAMEPLAY MECHANICS.** If a gameplay-changing rule is missing from this file or an approved task, report the ambiguity instead of inventing adjacency, movement, targeting, damage, matchmaking, or fallback behaviour.
