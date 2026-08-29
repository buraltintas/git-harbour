# GitHarbour product

GitHarbour is a web game built around one product object: a real GitHub contribution calendar. Its promise is **“Your GitHub history is a battlefield.”** A harbour is a frozen 10-week by 7-day slice of that calendar. Contribution intensity gives the board its visual identity but never changes game mechanics.

## Modes

- **Battle Your History (this vertical slice):** choose and defend one 10-week period; GitHarbour secretly chooses a different period from the same 52-week history; deploy, alternate shots with the AI, reveal the enemy dates at the end, and share the result.
- **Developer vs Developer (architecture only):** GitHub login, select a harbour, deploy, create a challenge link, opponent joins and deploys, server-authoritative alternating turns, result, PvP Elo, and sharing. There is no matchmaking or social-network layer.

## Solo journey

The signed-in (or local mock) player moves a 10-week window across a realistic contribution calendar, confirms a harbour, places five ships, and begins a persisted match. The enemy calendar pattern is visible, but its dates and ships stay hidden. Each player gets exactly one shot per turn. At completion the whole history returns with the enemy period highlighted, Solo rating and statistics update, and a result panel provides share copy and a public-share contract.

## Fairness and identity

Every cell retains date, weekday, contribution count, and contribution level. Counts and levels affect color only—not damage, probability, health, turns, placement, ranking, or AI. Selected boards are immutable snapshots, so later GitHub activity cannot change an existing game.

## Ratings, ranks, and statistics

Solo and PvP records are independent. Solo uses Elo against a configurable 1200-rated AI; PvP will use Elo between players. Ranks are Deckhand (<900), Sailor (900–1099), Officer (1100–1299), Commander (1300–1499), Captain (1500–1699), Admiral (1700–1899), and Fleet Admiral (1900+).

Each mode tracks games, wins, losses, win rate, rating, shots, hits, accuracy, current/longest streak, and average shots per completed win. Terminal updates are transactional and idempotent.

## Sharing

Results support 1200×630 images, public `/s/{shareId}` pages with server-rendered Open Graph/Twitter metadata, and `/share/users/{login}.svg` README statistics. Image rendering is replaceable behind interfaces; a placeholder implementation is sufficient in this slice.

