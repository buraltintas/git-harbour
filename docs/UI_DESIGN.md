# UI design

GitHarbour uses Primer controls and tokens with a restrained GitHub-adjacent visual language. Contribution grids are the game, not decorative terrain. There is no neon arcade treatment, fake 3D object, or unnecessary onboarding.

## Core Solo flow

`Dashboard → Start a history hunt → concealed 10×7 board → history reveal → share`

There is no date-range selection or deployment step. The dashboard has one primary action, and the server-selected game begins immediately. The active screen shows one board plus useful progress: targets found, total targets, shots, misses, and accuracy.

Unknown targets and empty cells are visually identical before selection. Hit cells reveal the authentic GitHub-like contribution level with a check glyph; misses use a neutral outlined state and dot. Color is never the only cue. Shot announcements use a polite live region and controls are busy/disabled while a request is in flight.

The result screen reveals the exact period, full frozen contribution slice, target and contribution totals, shots, misses, accuracy, rating change, share action, and another-hunt action. Copy uses “Harbour cleared”, “Contribution found”, and “Quiet day”.

## Accessibility and responsive behavior

The grid keeps a single roving tab stop, arrow-key navigation, disabled explored cells, coordinate-first accessible labels, visible focus, and reduced-motion support. Active unknown labels contain no hidden dates or contribution values. Approximately 390px layouts retain usable cell sizes through horizontal scrolling rather than squeezing 70 targets into tiny controls.

Light and dark themes must keep unknown/miss separation, contribution-green visibility, glyph contrast, focus rings, and readable status/progress text.

## Public and PvP surfaces

Public profile, README widget, canonical HTML, and Solo shares describe completed history hunts, targets found, shots, and accuracy. PvP history is labeled historical while new challenge/battle UI is gated behind a concise refit state. The challenge, polling, identity, and leaderboard shells remain reusable for the next contribution-target PvP phase.
