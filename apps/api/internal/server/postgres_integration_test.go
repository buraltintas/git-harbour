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

func TestPostgresPersistenceAndConcurrentFleetAction(t *testing.T) {
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
	playerUnits, err := game.RandomDeployment(player, game.SecureRand{})
	if err != nil {
		t.Fatal(err)
	}
	enemyUnits, err := game.RandomDeployment(enemy, game.SecureRand{})
	if err != nil {
		t.Fatal(err)
	}
	g := &State{ID: uuid(), Ruleset: game.ContributionFleetRuleset, Status: "battle", Turn: "player", PlayerBoard: player, EnemyBoard: enemy, PlayerDeployment: playerUnits, EnemyDeployment: enemyUnits, FleetActions: []game.FleetAction{}, PlayerStart: player[0].Date, EnemyStart: enemy[0].Date, Stats: decorate(PublicStats{Rating: 1200})}
	if err = repo.CreateGame(ctx, u.ID, g); err != nil {
		t.Fatal(err)
	}
	otherPool, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer otherPool.Close()
	restored, err := NewPostgresRepository(otherPool).Game(ctx, u.ID, g.ID)
	if err != nil || restored.ID != g.ID {
		t.Fatal("game did not survive repository restart", err)
	}
	attacker := playerUnits[0].Coord
	if _, events, err := NewPostgresRepository(otherPool).ActFleet(ctx, u.ID, g.ID, attacker, enemyUnits[0].Coord); err != nil || len(events) < 1 {
		t.Fatal("reciprocal transition did not persist", events, err)
	}
	restored, err = NewPostgresRepository(otherPool).Game(ctx, u.ID, g.ID)
	if err != nil || len(restored.PlayerBoard) != 70 || len(restored.EnemyBoard) != 70 || len(restored.FleetActions) < 1 {
		t.Fatal("snapshots, deployments, and actions did not survive restart", restored, err)
	}
	if restored.Status == "complete" {
		t.Skip("secure combat roll completed the small integration battle before concurrency assertion")
	}
	for _, unit := range restored.PlayerDeployment {
		if unit.Alive {
			attacker = unit.Coord
			break
		}
	}
	target := enemyUnits[1].Coord
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, _, e := NewPostgresRepository(otherPool).ActFleet(ctx, u.ID, g.ID, attacker, target)
			results <- e
		}()
	}
	success := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("duplicate concurrent target advanced %d times", success)
	}
	restored, err = NewPostgresRepository(otherPool).Game(ctx, u.ID, g.ID)
	if err != nil || len(restored.FleetActions) < 2 {
		t.Fatal("concurrent fleet state was not persisted safely", restored, err)
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
	for _, uid := range []string{winner.ID, loser.ID} {
		stats, statsErr := r.Stats(ctx, uid, "pvp")
		if statsErr != nil || stats.Games != 1 {
			t.Fatalf("stats for %s: %#v %v", uid, stats, statsErr)
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
