# GitHarbour rules

## Board and fleet

A battlefield is exactly 10 weeks horizontally by seven weekdays vertically: 70 cells. A snapshot cell contains `date`, `weekday` (0–6), `contributionCount`, and `contributionLevel` (0–4).

The fleet is Carrier (5), Battleship (4), Cruiser (3), Submarine (3), and Destroyer (2). Ships are straight, horizontal or vertical, in bounds, and non-overlapping. They may touch.

## Turns and victory

Opponents alternate one shot per turn. A hit gives no extra shot. A coordinate can be targeted only once. A ship sinks when every coordinate is hit; the first side to sink all five opposing ships wins. Finished games reject further actions.

In PvP, the server randomly chooses the opening player only after both immutable setups are locked. A challenge can be accepted once, cannot be self-accepted, expires after seven days, and reveals neither the opposing fleet nor its dates during play. A rematch is a new setup and is reserved for the previous opponent.

## Solo opponent

The server uses cryptographically secure randomness to choose a valid 10-week starting week from the same normalized history, excluding the player's starting week. Its range remains secret until the terminal result. The server also places its fleet with secure randomness.

## AI knowledge and behavior

The AI consumes only prior public shot outcomes, never player coordinates. It hunts among unexplored cells. Following a hit it targets valid orthogonal neighbors; aligned hits establish an orientation and extend along that line until sunk or exhausted. It never repeats a shot.

## Authority and integrity

The server accepts only intent: harbour start, fleet placement, or target coordinate. It validates placement, locks the game during a turn, derives hit/miss/sunk/victory, executes the AI response, and updates terminal ratings/statistics once in the same transaction. Hidden enemy fleet coordinates are never serialized in client projections.

Solo and PvP ratings/statistics are independent. A PvP result calculates both Elo changes from the two unchanged pre-match ratings, persists the two result rows atomically, and exposes only completed history publicly.
