# GitHarbour rules

`contribution_fleet_v3` is the active Solo ruleset. GitHub history is gameplay: activity breadth controls fleet size and activity intensity controls individual unit strength. Classic named ships, lengths, orientation, adjacency, ship health, and sunk semantics do not exist.

## Battlefield and period selection

A battlefield is a frozen, Sunday-aligned, contiguous `10 weeks × 7 weekdays = 70` real-calendar snapshot. Each cell retains date, weekday, contribution count, GitHub contribution level, derived Day Power, and derived combat level. The player may inspect every valid ten-week window, including zero-contribution windows, before locking one.

The preview keeps separate metrics: total contributions, active days, Contribution Power (the sum of all positive Day Powers), Fleet Capacity, peak day, and maximum deployable power. A positive day is only a deployment candidate; it is not automatically a unit.

## Central balance model

For `c > 0`:

```text
DayPower(c) = 1 + log2(1 + c)
```

Inactive days have zero contribution power. More contributions always increase power without a hard cap, while the logarithm provides diminishing returns.

Fleet Capacity uses active-day breadth:

```text
capacity = clamp(round(3 + sqrt(12 × activeDays / 7)), 3, 14)
```

This currently maps `0→3`, `1→4`, `10→7`, `30→10`, `50→12`, and `70→14`. Reserve Power is `1.0`. Human-readable combat levels are derived from continuous power: Reserve/zero=`0`, `<4`=`1`, `<7`=`2`, `<10`=`3`, otherwise `4`. Combat always uses continuous power, never the display level.

## Deployment

The player deploys exactly Fleet Capacity units before battle:

- If active days exceed capacity, the player chooses exactly `capacity` contribution-backed cells.
- If active days are at or below capacity, every active cell is required and empty slots are filled with Reserves.
- A contribution-backed unit stays on its real date coordinate and receives that day's Day Power.
- A Reserve uses an inactive real cell, has Power `1.0`, and never invents contribution activity.
- Units have no shapes, orientation, adjacency, clustering, or movement rules.
- Confirmation makes deployment immutable.

The computer applies the same capacity/power/Reserve rules and randomly chooses legal deployment cells server-side.

## Solo opponent period

The computer uses another valid ten-week period from the authenticated user's same real imported history. The server chooses randomly, preferring non-overlapping alternatives when available, and freezes it at game creation. Exactly one valid alternative is used normally. If no distinct valid alternative exists, creation stops with an explicit `history_not_playable` error; no synthetic data or fallback rule is invented.

## Combat

The player begins. Each action chooses one surviving deployed attacker and one opponent coordinate. A MISS or eliminated coordinate cannot be targeted again. A CLASH survivor remains exposed and may be targeted again; otherwise a defender win could make elimination impossible. A MISS contains no surviving deployed defender and eliminates nobody. A CLASH exposes both units and eliminates exactly one.

The attacker win probability is:

```text
P = clamp(1 / (1 + 10 ^ ((defenderPower - attackerPower) / 10)), 0.15, 0.85)
```

Equal power is `50%`; stronger power always improves the chance; neither side can reach certainty. The authoritative server records probability and secure random roll in persisted combat history. If the attacker wins, the defender is eliminated. Otherwise the attacker is eliminated. There are no HP bars or multi-hit units.

An attacker becomes exposed even after a miss. A defender becomes exposed when a clash occurs. During active play, an exposed enemy reveals only its coordinate and combat level—not its date, contribution count, exact power, or Reserve/contribution kind.

If neither fleet is eliminated after the player action, the computer performs exactly one equivalent action. It chooses among its own surviving attackers and uses only public combat observations: exposed enemy coordinates/levels, previous misses/clashes, and previously targeted coordinates. It never receives the hidden player deployment for target selection and calls no external AI service. Hits/clash wins never grant an extra action.

## Victory, reveal, and statistics

The first side whose opponent has zero surviving deployed units wins. Both Victory and Defeat are possible, including when an attacking unit loses the clash that eliminates its own side's last survivor.

Completion reveals both full frozen snapshots, both deployments, exact powers, combat history, period ranges, actions, misses, clashes won/lost, fleet sizes, surviving units, and rating change. Further actions are rejected.

V3 Solo stats are isolated from earlier rulesets. `shots` means player combat actions and `hits`/accuracy mean actions that produced a clash (clash rate), not contribution targets found. W/L, streaks, average actions per win, and 32-point Elo against the fixed 1200 computer update exactly once.

## Versioning and PvP

Completed `fleet_v1` and `contribution_targets_v2` records remain historical data and are never reinterpreted as v3. Their legacy fields may remain in persisted JSON/schema solely for compatibility. New v3 Solo uses separate ruleset stats. PvP is explicitly outside this task and remains gated/archived.

## Contributor invariant

**NEVER INFER UNSPECIFIED GAMEPLAY MECHANICS.** If a gameplay-changing rule is missing from this file or the approved task, report/ask about the ambiguity instead of inventing adjacency, movement, targeting, damage, matchmaking, fallback, or other behavior.
