package server

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/githarbour/githarbour/apps/api/internal/game"
	"net/http/httptest"
	"testing"
)

func TestHandlerRoutesBuild(t *testing.T) {
	s := New(context.Background())
	if s.Handler() == nil {
		t.Fatal("handler is nil")
	}
}

func TestWrongTurnAndTerminal(t *testing.T) {
	s := New(context.Background())
	g := &State{ID: "x", Status: "battle", Turn: "ai"}
	s.games["x"] = g
	r := httptest.NewRequest("POST", "/v1/games/x/shots", bytes.NewBufferString(`{"x":0,"y":0}`))
	r.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	s.shot(w, r)
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	g.Status = "complete"
	g.Turn = "player"
	w = httptest.NewRecorder()
	s.shot(w, r)
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
}
func TestStatsOnlyUpdateOnce(t *testing.T) {
	s := New(context.Background())
	g := &State{Stats: PublicStats{Rating: 1200}, PlayerShots: []game.Shot{{Result: "hit"}}}
	s.stats = PublicStats{Rating: 1200}
	s.finish(g, "player")
	first := s.stats
	s.finish(g, "player")
	if s.stats.Games != first.Games || s.stats.Rating != first.Rating {
		t.Fatal("updated twice")
	}
}
func TestPublicHidesEnemy(t *testing.T) {
	g := &State{EnemyStart: "2025-01-01", EnemyFleet: []game.Ship{{Kind: "Destroyer"}}}
	b, _ := json.Marshal(public(g))
	if bytes.Contains(b, []byte("2025-01-01")) || bytes.Contains(b, []byte("Destroyer")) {
		t.Fatal(string(b))
	}
}
