# GitHarbour product

GitHarbour’s core object is a real public GitHub contribution calendar: **“Your GitHub history is a battlefield.”** Activity supplies visual identity only and never affects damage, probability, health, turns, or rating.

GitHarbour ships two complete modes. **Battle Your History** pits a frozen 10-week harbour against the authoritative Solo AI. **Developer vs Developer** creates a private challenge link; two GitHub users independently freeze boards/fleets and then alternate asynchronous, server-authoritative turns. Refreshing contribution imports cannot mutate either mode's snapshots.

Every signed-in developer also gets:

- an interactive Pages profile at `#/u/{login}`;
- canonical crawlable API HTML at `/u/{login}`;
- safe public Solo/PvP projections;
- a light/dark GitHub README SVG widget;
- stable completed-battle shares and 1200×630 PvP result cards;
- a public PvP leaderboard and completed-match history;
- a rematch link reserved for the prior opponent.

GitHub login is the only production authentication method. Public pages exist only after that GitHub user has joined GitHarbour; arbitrary usernames are not auto-created.

Matchmaking, chat, feeds, friends, push notifications, WebSockets and Supabase Realtime remain outside the product. The asynchronous experience uses bounded polling while a battle or challenge is on screen.
