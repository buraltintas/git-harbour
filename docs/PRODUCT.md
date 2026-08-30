# GitHarbour product

GitHarbour’s core object is a real public GitHub contribution calendar: **“Your GitHub history is a battlefield.”** Contribution cells are the targets. There is no separate ship placement layer, and contribution intensity never adds health, damage, probability, or extra turns.

## Battle Your History

Solo is a reciprocal battle between two frozen periods from the authenticated developer's own GitHub history.

The player first chooses a playable contiguous ten-week period as **Your Harbour**. Its dates and contribution pattern remain visible during battle. The server securely matches a different period with the closest practical target count as the **Hidden Harbour**, preferring a non-overlapping period when fairness permits. Its dates and untouched target positions remain concealed.

The player fires one shot. If the Hidden Harbour survives, the non-cheating AI randomly selects one unshot coordinate and fires once at the Player Harbour. A hit never grants an extra shot. The player wins by finding all hidden contribution days first; the AI wins by finding all player contribution days first. Completion reveals both periods, Victory/Defeat, shots, accuracy, Elo change, and a stable share.

Solo statistics represent competitive games: games, wins, losses, win rate, fixed-1200-opponent Elo, player shots/hits/accuracy, current and longest win streak, and average player shots per victory. AI shots never affect player accuracy.

## Developer vs Developer

The existing challenge, identity, rating, history, and leaderboard infrastructure remains preserved. New PvP creation is temporarily gated. Its next ruleset will reuse reciprocal Solo directly: each developer chooses a contribution harbour, takes one shot per turn, and wins by finding every target on the opponent's frozen board.

## Preserved platform

GitHub OAuth, application sessions, contribution import, Supabase PostgreSQL/pgx, public profiles, separate Solo/PvP stats, stable share IDs, canonical HTML, README widgets, social cards, GitHub Pages, Koyeb deployment, Primer styling, and authorization boundaries remain intact.

Classic named ships, deployment controls, orientation, and ship health remain retired. Migration 007 archives the temporary one-sided v2 history-hunt counters and gives reciprocal Solo a clean W/L and Elo baseline without discarding the audit record.
