# GitHarbour rules

## Board and targets

A harbour is exactly ten weeks by seven weekdays: 70 frozen cells. Each internal snapshot cell contains its date, weekday, contribution count, and contribution level.

- `contributionCount > 0` is one contribution target.
- `contributionCount == 0` is empty water.
- One hit clears a target regardless of count or intensity.
- Contribution level is visual information revealed after a hit and at completion.

There is no manual deployment, orientation, named ship, reciprocal Solo opponent, or client-supplied target pattern.

## Hidden play and victory

The player submits only a coordinate. An unexplored active cell exposes no date, weekday, count, level, or target state. A hit reveals that cell’s frozen count and level; a miss reveals only a quiet day. Dates remain hidden until completion.

The harbour clears immediately after every contribution target has been hit. Empty cells do not need to be fired upon. Duplicate, out-of-bounds, and post-completion shots are rejected.

## Solo window selection

The server enumerates week-aligned, contiguous 70-day windows. It prefers windows containing 10–45 target days and chooses randomly among eligible candidates. If none meet the ideal density, it chooses randomly among the non-empty windows closest to that range. An all-zero history cannot produce a meaningful game and returns a friendly unavailable result.

The selected snapshot is copied into the game. Later contribution imports cannot mutate active or completed games.

## Solo performance

Solo has no loss race or shot budget. A completion records targets, shots, misses, accuracy, total contributions, and rating delta exactly once. The density-aware rating delta is based on the expected location of the last target in a random ordering, scaled by three and bounded to −12…+12.

## Future PvP

Each player will choose an eligible ten-week contribution slice. One shot is taken per turn, including after a hit. The first player to find every target on the opponent’s board wins. Exact periods remain hidden until the game completes. The same default 10–45 target quality range applies to both players.

Legacy `fleet_v1` prototype games remain distinguishable in persistence and are not compatible with the `contribution_targets_v2` ruleset.
