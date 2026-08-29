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

func TestPostgresPersistenceAndConcurrentShot(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; live PostgreSQL integration not run")
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
	fleet := []game.Ship{}
	for y, spec := range []struct {
		Name string
		Size int
	}{{"Carrier", 5}, {"Battleship", 4}, {"Cruiser", 3}, {"Submarine", 3}, {"Destroyer", 2}} {
		cells := []game.Coord{}
		for x := 0; x < spec.Size; x++ {
			cells = append(cells, game.Coord{X: x, Y: y})
		}
		fleet = append(fleet, game.Ship{Kind: spec.Name, Cells: cells})
	}
	g := &State{ID: uuid(), Status: "battle", Turn: "player", PlayerStart: "2025-01-01", EnemyStart: "2025-04-01", PlayerFleet: fleet, EnemyFleet: fleet, PlayerShots: []game.Shot{}, AIShots: []game.Shot{}, Stats: decorate(PublicStats{Rating: 1200})}
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
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, _, e := NewPostgresRepository(otherPool).Shoot(ctx, u.ID, g.ID, game.Coord{X: 9, Y: 6})
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
		t.Fatalf("duplicate concurrent shot advanced %d times", success)
	}
}
