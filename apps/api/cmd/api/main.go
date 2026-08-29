package main

import (
	"context"
	"github.com/githarbour/githarbour/apps/api/internal/server"
	"log"
	"net/http"
	"os"
)

func main() {
	s, err := server.New(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("GitHarbour API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, s.Handler()))
}
