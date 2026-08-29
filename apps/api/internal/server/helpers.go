package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
var ErrExpired = errors.New("expired or consumed")
var ErrConflict = errors.New("conflict")

func randomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func digest(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }
func writeJSON(w http.ResponseWriter, n int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(n)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, n int, code, msg string) {
	writeJSON(w, n, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
func decorate(p PublicStats) PublicStats {
	if p.Rating == 0 {
		p.Rating = 1200
	}
	if p.Games > 0 {
		p.WinRate = 100 * float64(p.Wins) / float64(p.Games)
	}
	if p.Shots > 0 {
		p.Accuracy = 100 * float64(p.Hits) / float64(p.Shots)
	}
	if p.Wins > 0 {
		p.AverageShotsPerWin = float64(p.WinShots) / float64(p.Wins)
	}
	switch {
	case p.Rating < 900:
		p.Rank = "Deckhand"
	case p.Rating < 1100:
		p.Rank = "Sailor"
	case p.Rating < 1300:
		p.Rank = "Officer"
	case p.Rating < 1500:
		p.Rank = "Commander"
	case p.Rating < 1700:
		p.Rank = "Captain"
	case p.Rating < 1900:
		p.Rank = "Admiral"
	default:
		p.Rank = "Fleet Admiral"
	}
	return p
}
func publicGame(g *State) map[string]any {
	b, _ := json.Marshal(g)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	delete(m, "enemyFleet")
	delete(m, "enemyStart")
	if g.Status == "complete" {
		m["enemyPeriod"] = map[string]string{"start": g.EnemyStart, "end": dateEnd(g.EnemyStart)}
	}
	return m
}
func dateEnd(v string) string {
	d, _ := time.Parse("2006-01-02", v)
	return d.AddDate(0, 0, 69).Format("2006-01-02")
}
func stripDates(c []game.Cell) []game.Cell {
	o := append([]game.Cell(nil), c...)
	for i := range o {
		o[i].Date = ""
	}
	return o
}
func countHits(v []game.Shot) int {
	n := 0
	for _, s := range v {
		if s.Result != "miss" {
			n++
		}
	}
	return n
}
func finishState(g *State, p PublicStats, winner string) (PublicStats, string) {
	if g.TerminalApplied {
		return p, g.ShareID
	}
	g.Status = "complete"
	g.Turn = "complete"
	g.Winner = winner
	g.TerminalApplied = true
	old := p.Rating
	gs := game.Stats{Games: p.Games, Wins: p.Wins, Losses: p.Losses, Rating: p.Rating, Shots: p.Shots, Hits: p.Hits, CurrentStreak: p.CurrentStreak, LongestStreak: p.LongestStreak, WinShots: p.WinShots}
	gs = game.UpdateStats(gs, winner == "player", len(g.PlayerShots), countHits(g.PlayerShots))
	p = decorate(PublicStats{Games: gs.Games, Wins: gs.Wins, Losses: gs.Losses, Rating: gs.Rating, Shots: gs.Shots, Hits: gs.Hits, CurrentStreak: gs.CurrentStreak, LongestStreak: gs.LongestStreak, WinShots: gs.WinShots})
	g.Stats = p
	g.RatingDelta = p.Rating - old
	g.ShareID = randomSecret(9)
	return p, g.ShareID
}
func resolveTurn(g *State, p PublicStats, c game.Coord) ([]game.Shot, PublicStats, error) {
	if g.Status == "complete" {
		return nil, p, errors.New("game is complete")
	}
	if g.Turn != "player" {
		return nil, p, errors.New("wrong turn")
	}
	ev, n, e := game.ResolveShot(g.EnemyFleet, g.PlayerShots, c)
	if e != nil {
		return nil, p, e
	}
	g.EnemyFleet = n
	g.PlayerShots = append(g.PlayerShots, ev)
	events := []game.Shot{ev}
	if game.AllSunk(g.EnemyFleet) {
		p, _ = finishState(g, p, "player")
		return events, p, nil
	}
	g.Turn = "ai"
	target, e := game.NextAITarget(g.AIShots, game.SecureRand{})
	if e != nil {
		return nil, p, e
	}
	a, pf, e := game.ResolveShot(g.PlayerFleet, g.AIShots, target)
	if e != nil {
		return nil, p, e
	}
	g.PlayerFleet = pf
	g.AIShots = append(g.AIShots, a)
	events = append(events, a)
	if game.AllSunk(g.PlayerFleet) {
		p, _ = finishState(g, p, "ai")
	} else {
		g.Turn = "player"
	}
	return events, p, nil
}
func escape(v string) string { return html.EscapeString(v) }
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}
