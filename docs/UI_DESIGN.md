# UI design

## Solo journey

`Login → inspect 10-week periods → lock period → deploy contribution/Reserve units → confirm → battle → Victory/Defeat reveal → share`

Period selection keeps the GitHub contribution graph primary and shows date range, total contributions, active days, Contribution Power, Fleet Capacity, peak day, and maximum deployable power. Zero-activity periods remain selectable.

Deployment distinguishes eligible real activity, selected contribution units, mandatory active units, inactive Reserve positions, continuous power, and display level. Contribution units stay on real coordinates. Confirmation is explicitly immutable.

Battle shows **Enemy Harbour** first and **My Harbour** second. Each turn requires selecting an alive unit on My Harbour, then an untargeted enemy coordinate. The banner distinguishes player resolution and computer response. Unknown, deployed, selected, exposed, eliminated, and already-targeted states have text/ARIA equivalents; color is not the only signal.

Desktop uses two columns. Mobile stacks Enemy Harbour first and preserves horizontal grid scrolling rather than shrinking touch targets. Keyboard roving focus and reduced-motion timing remain supported.

The result reveals both exact periods and deployments. It shows W/L, actions, misses, clashes won/lost, starting fleet sizes, Contribution Power, survivors, turns, rating delta/rank, share, and replay.

## Public surfaces

Public profile, widget, canonical HTML, and v3 share use competitive W/L, rating, actions, and clash rate terminology. They never use classic ship names or describe positive days as automatic targets. PvP remains historical/gated.
