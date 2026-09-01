# UI design

## Solo journey

`Login → inspect 10-week periods → lock period → deploy contribution/Reserve units → confirm → battle → Victory/Defeat reveal → share`

Period selection keeps the GitHub contribution graph primary and shows date range, total contributions, active days, bounded Fleet Capacity, and peak day. Zero-activity periods remain selectable.

Deployment distinguishes eligible real activity, selected contribution units, mandatory active units, and inactive Reserve positions. Contribution units stay on real coordinates. Confirmation is explicitly immutable.

Battle shows **My Harbour** on the left and **Enemy Harbour** on the right. My Harbour is read-only during combat. Each turn requires only one click on an untargeted enemy coordinate. The banner distinguishes player resolution and computer response. Unknown, deployed, HIT/eliminated, and MISS states have text/ARIA equivalents; color is not the only signal.

Desktop uses two columns. Mobile stacks the boards and preserves horizontal grid scrolling rather than shrinking touch targets. Keyboard roving focus and reduced-motion timing remain supported.

The result reveals both exact periods and deployments. It shows W/L, hits, misses, starting fleet sizes, survivors, turns, rating delta/rank, share, and replay.

## Public surfaces

Public profile, widget, canonical HTML, and v4 share use competitive W/L, rating, shots, and accuracy terminology. They never use classic ship names or describe positive days as automatic targets. PvP remains historical/gated.
