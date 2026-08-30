# GitHarbour rules

## Contribution targets

Each harbour is ten weeks by seven weekdays: 70 frozen cells. Every snapshot cell contains its date, weekday, contribution count, and contribution level.

- `contributionCount > 0` is one hittable target.
- `contributionCount == 0` is empty water and produces a miss.
- One hit clears a target regardless of contribution count or intensity.
- Contribution level is visual only.
- There are no named ships, orientation controls, or deployment step.

## Reciprocal Solo setup

The authenticated player chooses a week-aligned, contiguous ten-week period from their imported GitHub history. The server validates and freezes that period as the Player Harbour. It then chooses a different playable ten-week window from the same history as the AI Harbour.

If normal 10–45-target player windows exist, selection is limited to that quality range; sparse or unusually dense histories fall back only to windows closest to it. Opponent selection prefers non-overlapping windows within five target days, then minimizes target-count difference and uses secure randomness among exact ties. If no fair non-overlapping period exists, it applies the same closest-density rule across all different windows. Both snapshots are immutable after creation.

## Turn and AI model

The player starts. A transition is authoritative and transactional:

1. The player fires one coordinate at the hidden AI Harbour.
2. If all AI targets are hit, the player wins immediately.
3. Otherwise the AI selects one coordinate not present in its prior shots and fires at the Player Harbour.
4. If all player targets are hit, the AI wins; otherwise control returns to the player.

A hit never grants an extra shot. The AI selector receives only board dimensions and its previous shot coordinates/results; it never receives either frozen board and cannot inspect hidden targets.

## Visibility and completion

The player sees their selected harbour, dates, contribution pattern, and AI hit/miss marks during battle. On the AI Harbour, an untouched cell exposes only its coordinate and `unknown` state. A hit reveals frozen contribution count/level and a miss reveals only empty water. Enemy dates and the exact enemy period remain hidden until completion.

The first side to hit all contribution targets on the opposing harbour wins. Empty cells do not need to be explored. Terminal results use `player` or `ai`, reveal both periods, and reject further shots.

## Solo statistics

Completed reciprocal games update games, wins, losses, win rate, current/longest win streak, player shots, player hits, accuracy, and average player shots per win exactly once. AI shots never enter player accuracy. Solo rating uses the existing 32-point Elo helper against a fixed 1200-rated AI.

The migration archives temporary one-sided v2 history-hunt counters before resetting reciprocal Solo statistics to a clean semantic baseline. Legacy `fleet_v1` games remain distinguishable and are never reinterpreted.

## Future PvP

Future PvP replaces the AI Harbour with another authenticated developer's selected harbour and replaces the synchronous AI response with the opponent's turn. The two-board contribution-target rules and first-to-find-all-targets win condition remain unchanged.
