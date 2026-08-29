package main

import (
	"context"
	"github.com/githarbour/githarbour/apps/api/internal/server"
	"log"
	"net/http"
	"os"
)

func main() {
	s := server.New(context.Background())
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("GitHarbour API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, s.Handler()))
}
