package main

import (
	"context"
	"log"
	"os"

	"github.com/githarbour/githarbour/apps/api/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "up" {
		log.Fatal("usage: go run ./cmd/migrate up")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}
	if err = migrations.Up(ctx, pool, dir); err != nil {
		log.Fatal(err)
	}
	log.Print("migrations applied")
}
