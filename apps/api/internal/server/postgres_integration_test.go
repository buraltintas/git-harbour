package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
	"github.com/githarbour/githarbour/apps/api/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresFleetV3RetirementAndPersistence(t *testing.T) {
	url := os.Getenv("GITHARBOUR_INTEGRATION_DATABASE_URL")
	if url == "" {
		t.Skip("GITHARBOUR_INTEGRATION_DATABASE_URL not set; isolated PostgreSQL integration not run")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 3
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Up(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(pool)
	login := "integration-" + randomSecret(6)
	u, err := repo.UpsertGitHubUser(ctx, User{GitHubID: time.Now().UnixNano(), Login: login, Name: "Integration"}, mockCells(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	days := mockCells(time.Now())
	player, enemy := days[:70], days[70:140]
	// contribution_fleet_v3 was retired by the v4 battleship restore (commit
	// d47f59e). An in-progress fleet game must no longer load through Game(), while
	// a completed one stays readable so historical results survive a restart.
	inProgress := &State{ID: uuid(), Ruleset: game.ContributionFleetRuleset, Status: "battle", Turn: "player", PlayerBoard: player, EnemyBoard: enemy, PlayerStart: player[0].Date, EnemyStart: enemy[0].Date, Stats: decorate(PublicStats{Rating: 1200})}
	if err = repo.CreateGame(ctx, u.ID, inProgress); err != nil {
		t.Fatal(err)
	}
	otherPool, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer otherPool.Close()
	restarted := NewPostgresRepository(otherPool)
	if _, err = restarted.Game(ctx, u.ID, inProgress.ID); err != ErrLegacyGame {
		t.Fatalf("in-progress fleet_v3 must be retired across restart, got %v", err)
	}
	completed := &State{ID: uuid(), Ruleset: game.ContributionFleetRuleset, Status: "complete", Turn: "complete", Winner: "player", PlayerBoard: player, EnemyBoard: enemy, PlayerStart: player[0].Date, EnemyStart: enemy[0].Date, Stats: decorate(PublicStats{Rating: 1200})}
	if err = repo.CreateGame(ctx, u.ID, completed); err != nil {
		t.Fatal(err)
	}
	restored, err := restarted.Game(ctx, u.ID, completed.ID)
	if err != nil || restored.ID != completed.ID || len(restored.PlayerBoard) != 70 || len(restored.EnemyBoard) != 70 {
		t.Fatalf("completed fleet_v3 must stay readable across restart: %#v %v", restored, err)
	}
}

func TestPostgresConcurrentChallengeAcceptance(t *testing.T) {
	url := os.Getenv("GITHARBOUR_INTEGRATION_DATABASE_URL")
	if url == "" {
		t.Skip("GITHARBOUR_INTEGRATION_DATABASE_URL not set; isolated PostgreSQL integration not run")
	}
	ctx := context.Background()
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.MaxConns = 4
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if e = migrations.Up(ctx, pool, "../../migrations"); e != nil {
		t.Fatal(e)
	}
	r := NewPostgresRepository(pool)
	seed := time.Now().UnixNano()
	a, e := r.UpsertGitHubUser(ctx, User{GitHubID: seed, Login: "pvp-a-" + randomSecret(5)}, mockCells(time.Now()))
	if e != nil {
		t.Fatal(e)
	}
	b, e := r.UpsertGitHubUser(ctx, User{GitHubID: seed + 1, Login: "pvp-b-" + randomSecret(5)}, mockCells(time.Now()))
	if e != nil {
		t.Fatal(e)
	}
	c, e := r.UpsertGitHubUser(ctx, User{GitHubID: seed + 2, Login: "pvp-c-" + randomSecret(5)}, mockCells(time.Now()))
	if e != nil {
		t.Fatal(e)
	}
	ch, e := r.CreateChallenge(ctx, a.ID, randomSecret(12), time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, uid := range []string{b.ID, c.ID} {
		go func(id string) {
			<-start
			_, _, x := NewPostgresRepository(pool).AcceptChallenge(ctx, id, ch.Code, time.Now())
			results <- x
		}(uid)
	}
	close(start)
	success := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("acceptance winners=%d", success)
	}
	var games int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM games WHERE id=(SELECT game_id FROM challenges WHERE code=$1)`, ch.Code).Scan(&games); e != nil || games != 1 {
		t.Fatalf("game count=%d err=%v", games, e)
	}
}

func TestPostgresPVPFullLifecycleAndTerminalRace(t *testing.T) {
	url := os.Getenv("GITHARBOUR_INTEGRATION_DATABASE_URL")
	if url == "" {
		t.Skip("GITHARBOUR_INTEGRATION_DATABASE_URL not set; isolated PostgreSQL integration not run")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 5
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Up(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	r := NewPostgresRepository(pool)
	seed := time.Now().UnixNano()
	days := mockCells(time.Now())
	a, err := r.UpsertGitHubUser(ctx, User{GitHubID: seed, Login: "flow-a-" + randomSecret(5)}, days)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.UpsertGitHubUser(ctx, User{GitHubID: seed + 1, Login: "flow-b-" + randomSecret(5)}, days)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := r.CreateChallenge(ctx, a.ID, randomSecret(12), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.AcceptChallenge(ctx, b.ID, ch.Code, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{a.ID, b.ID} {
		rows, listErr := r.Battles(ctx, uid)
		if listErr != nil || len(rows) != 1 || rows[0].Status != "setup" {
			t.Fatalf("setup battle missing for %s: %#v %v", uid, rows, listErr)
		}
	}
	if _, _, err = r.ReadyChallenge(ctx, a.ID, ch.Code, days[0].Date, days[:70], pvpFleet(), time.Now()); err != nil {
		t.Fatal(err)
	}
	_, ready, err := r.ReadyChallenge(ctx, b.ID, ch.Code, days[0].Date, days[:70], pvpFleet(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	winner, loser := a, b
	if ready.CurrentTurn == b.ID {
		winner, loser = b, a
	}
	targets := []game.Coord{}
	for _, ship := range pvpFleet() {
		targets = append(targets, ship.Cells...)
	}
	misses := []game.Coord{{X: 9, Y: 6}, {X: 8, Y: 6}, {X: 7, Y: 6}, {X: 6, Y: 6}, {X: 5, Y: 6}, {X: 4, Y: 6}, {X: 3, Y: 6}, {X: 2, Y: 6}, {X: 1, Y: 6}, {X: 0, Y: 6}, {X: 9, Y: 5}, {X: 8, Y: 5}, {X: 7, Y: 5}, {X: 6, Y: 5}, {X: 5, Y: 5}, {X: 4, Y: 5}}
	for i := 0; i < len(targets)-1; i++ {
		if _, _, err = r.ShootPVP(ctx, winner.ID, ready.ID, targets[i]); err != nil {
			t.Fatalf("winner shot %d: %v", i, err)
		}
		if _, _, err = r.ShootPVP(ctx, loser.ID, ready.ID, misses[i]); err != nil {
			t.Fatalf("loser shot %d: %v", i, err)
		}
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, _, shotErr := NewPostgresRepository(pool).ShootPVP(ctx, winner.ID, ready.ID, targets[len(targets)-1])
			results <- shotErr
		}()
	}
	close(start)
	success := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("terminal winners=%d", success)
	}
	var resultRows, shareRows int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pvp_results WHERE game_id=$1`, ready.ID).Scan(&resultRows); err != nil || resultRows != 2 {
		t.Fatalf("results=%d err=%v", resultRows, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM shares WHERE game_id=$1`, ready.ID).Scan(&shareRows); err != nil || shareRows != 1 {
		t.Fatalf("shares=%d err=%v", shareRows, err)
	}
	// Legacy challenge PvP records into mode_stats and is isolated from the active
	// v4 PvP stats surfaced by Stats(pvp), which reads ruleset_mode_stats.
	for _, uid := range []string{winner.ID, loser.ID} {
		var legacyGames int
		if err = pool.QueryRow(ctx, `SELECT games FROM mode_stats WHERE user_id=$1 AND mode='pvp'`, uid).Scan(&legacyGames); err != nil || legacyGames != 1 {
			t.Fatalf("legacy pvp stats for %s: games=%d err=%v", uid, legacyGames, err)
		}
		if v4, statsErr := r.Stats(ctx, uid, "pvp"); statsErr != nil || v4.Games != 0 {
			t.Fatalf("legacy pvp must not feed v4 stats for %s: %#v %v", uid, v4, statsErr)
		}
	}
	first, err := r.Rematch(ctx, winner.ID, ready.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Rematch(ctx, winner.ID, ready.ID, time.Now().Add(time.Hour))
	if err != nil || first.Code != second.Code {
		t.Fatalf("rematch first=%s second=%s err=%v", first.Code, second.Code, err)
	}
}

// TestPostgresAsyncPVPFullLifecycle drives an asynchronous open-harbour battle
// end to end against real PostgreSQL. It guards the jsonb-cast terminal UPDATE in
// ShootAsyncPVP: an untyped `$2->>'pvpDefenderId'` made every shot fail with
// "operator is not unique" (SQLSTATE 42725), so no async battle could complete.
func TestPostgresAsyncPVPFullLifecycle(t *testing.T) {
	url := os.Getenv("GITHARBOUR_INTEGRATION_DATABASE_URL")
	if url == "" {
		t.Skip("GITHARBOUR_INTEGRATION_DATABASE_URL not set; isolated PostgreSQL integration not run")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Up(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	r := NewPostgresRepository(pool)
	now := time.Now().UTC()
	days := mockCells(now)
	seed := now.UnixNano()
	alice, err := r.UpsertGitHubUser(ctx, User{GitHubID: seed, Login: "async-a-" + randomSecret(5)}, days)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := r.UpsertGitHubUser(ctx, User{GitHubID: seed + 1, Login: "async-b-" + randomSecret(5)}, days)
	if err != nil {
		t.Fatal(err)
	}
	window, err := game.FleetWindowAt(days, days[0].Date)
	if err != nil {
		t.Fatal(err)
	}
	units, err := game.ValidateDeployment(window.Cells, deploymentChoices(window.Cells))
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []User{alice, bob} {
		if _, err = r.SetOpenHarbour(ctx, u.ID, window.Cells[0].Date, window.Cells, units, now); err != nil {
			t.Fatalf("open harbour for %s: %v", u.Login, err)
		}
	}
	// Only distinct open harbours are challengeable.
	open, err := r.OpenHarbours(ctx, alice.ID)
	if err != nil || len(open) != 1 || open[0].Owner.ID != bob.ID {
		t.Fatalf("open harbours projection: %#v %v", open, err)
	}
	battle, err := r.StartAsyncPVP(ctx, alice.ID, bob.Login, now)
	if err != nil {
		t.Fatal(err)
	}
	gid := battle.State.ID
	if battle.State.PVPDefenderID != bob.ID || battle.State.Turn != "player" {
		t.Fatalf("unexpected battle roles: %#v", battle.State)
	}
	// Sweep every coordinate. This survives the automated defender winning first;
	// whichever side reaches zero, the terminal UPDATE must parse and commit.
	var final *AsyncPVPBattle
	for i := 0; i < game.BoardCells && final == nil; i++ {
		c := game.Coord{X: i / game.Height, Y: i % game.Height}
		updated, events, shotErr := r.ShootAsyncPVP(ctx, alice.ID, gid, c)
		if shotErr == ErrGameComplete {
			break
		}
		if shotErr != nil {
			t.Fatalf("shot at %+v failed: %v", c, shotErr)
		}
		if len(events) == 0 || events[0].Actor != "player" {
			t.Fatalf("expected a challenger event first, got %#v", events)
		}
		if updated.State.Status == "complete" {
			final = updated
		}
	}
	if final == nil {
		t.Fatal("battle never completed after sweeping the board")
	}
	if final.State.Winner != "player" && final.State.Winner != "ai" {
		t.Fatalf("no winner recorded: %q", final.State.Winner)
	}
	alive := game.AliveCount(final.State.PlayerDeployment) + game.AliveCount(final.State.EnemyDeployment)
	if game.AliveCount(final.State.PlayerDeployment) > 0 && game.AliveCount(final.State.EnemyDeployment) > 0 {
		t.Fatalf("both fleets still alive at completion: %d", alive)
	}
	if final.State.ShareID == "" {
		t.Fatal("completed battle has no share id")
	}
	// Terminal stats, results, share, and leaderboard must be recorded once for both.
	var resultRows, shareRows int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pvp_results WHERE game_id=$1`, gid).Scan(&resultRows); err != nil || resultRows != 2 {
		t.Fatalf("pvp_results=%d err=%v", resultRows, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM shares WHERE game_id=$1`, gid).Scan(&shareRows); err != nil || shareRows != 1 {
		t.Fatalf("shares=%d err=%v", shareRows, err)
	}
	for _, u := range []User{alice, bob} {
		stats, statsErr := r.Stats(ctx, u.ID, "pvp")
		if statsErr != nil || stats.Games != 1 {
			t.Fatalf("pvp stats for %s: %#v %v", u.Login, stats, statsErr)
		}
		history, histErr := r.PVPHistory(ctx, u.ID, 10)
		if histErr != nil || len(history) != 1 || history[0].GameID != gid {
			t.Fatalf("pvp history for %s: %#v %v", u.Login, history, histErr)
		}
	}
	board, err := r.Leaderboard(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range board {
		seen[e.Login] = true
	}
	if !seen[alice.Login] || !seen[bob.Login] {
		t.Fatalf("leaderboard missing async participants: %#v", board)
	}
	// A further shot on the finished battle is rejected without doubling stats.
	if _, _, err = r.ShootAsyncPVP(ctx, alice.ID, gid, game.Coord{X: 0, Y: 0}); err != ErrGameComplete {
		t.Fatalf("post-completion shot expected ErrGameComplete, got %v", err)
	}
}
