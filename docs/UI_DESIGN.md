# UI design

GitHarbour should feel at home beside GitHub without copying it. Primer supplies controls, tokens, icons, theme behavior, spacing, and focus states. The identity comes from harbour language and contribution-grid gameplay—not glass, neon, oversized display type, or decorative spectacle.

## Layout and states

A compact masthead contains the mark, mode label, theme toggle, and player identity. The main content uses a readable centered width and clear progression: **Choose harbour → Deploy fleet → Battle → Result**. On desktop, contextual status sits beside the board; on mobile it stacks below with at least 44px touch targets.

The contribution calendar keeps weekdays vertical and weeks horizontal. Counts are available through accessible labels/tooltips; five familiar contribution levels use theme-aware green tokens. A crisp outlined 10-week window moves via previous/next controls, keyboard arrows, range input, and direct week selection. The selection summary always states exact dates.

Deployment shows the whole selected contribution pattern, an explicit fleet list, orientation toggle, selected ship, valid/invalid placement preview, and undo/reset actions. It must work with pointer, touch, and keyboard buttons.

Battle presents two 10×7 boards: the player's harbour with ships and incoming AI shots, and the enemy contribution pattern with only the player's shot outcomes. Hit, miss, sunk, active turn, and remaining ships are distinguishable by color plus icon/text—not color alone. Motion is brief and disabled by `prefers-reduced-motion`.

The result prioritizes win/loss, rating delta, revealed historical period, stats, and share copy. The full 52-week calendar clearly outlines the formerly hidden enemy range. Error, loading, empty, and reconnect states use plain, actionable copy.

## Accessibility and responsive behavior

Semantic landmarks, visible labels, status announcements, keyboard focus, AA contrast, reduced motion, and non-color state cues are required. Dense 52-week history may scroll horizontally on small screens; playable boards scale to the viewport without forcing tiny tap targets.

