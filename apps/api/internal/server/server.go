package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/githarbour/githarbour/apps/api/internal/game"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type State struct {
	ID              string      `json:"id"`
	Status          string      `json:"status"`
	Turn            string      `json:"turn"`
	PlayerBoard     []game.Cell `json:"playerBoard"`
	EnemyBoard      []game.Cell `json:"enemyBoard"`
	PlayerFleet     []game.Ship `json:"playerFleet"`
	EnemyFleet      []game.Ship `json:"-"`
	PlayerShots     []game.Shot `json:"playerShots"`
	AIShots         []game.Shot `json:"aiShots"`
	PlayerStart     string      `json:"playerStart"`
	EnemyStart      string      `json:"-"`
	Winner          string      `json:"winner,omitempty"`
	RatingDelta     int         `json:"ratingDelta,omitempty"`
	ShareID         string      `json:"shareId,omitempty"`
	Stats           PublicStats `json:"stats"`
	TerminalApplied bool        `json:"-"`
}
type PublicStats struct {
	Games              int     `json:"games"`
	Wins               int     `json:"wins"`
	Losses             int     `json:"losses"`
	Rating             int     `json:"rating"`
	Shots              int     `json:"shots"`
	Hits               int     `json:"hits"`
	CurrentStreak      int     `json:"currentStreak"`
	LongestStreak      int     `json:"longestStreak"`
	WinShots           int     `json:"-"`
	WinRate            float64 `json:"winRate"`
	Accuracy           float64 `json:"accuracy"`
	AverageShotsPerWin float64 `json:"averageShotsPerWin"`
	Rank               string  `json:"rank"`
}
type Server struct {
	mu        sync.Mutex
	games     map[string]*State
	stats     PublicStats
	db        *pgxpool.Pool
	webOrigin string
}

func New(ctx context.Context) *Server {
	s := &Server{games: map[string]*State{}, stats: PublicStats{Rating: 1200}, webOrigin: env("WEB_ORIGIN", "http://localhost:5173")}
	if u := os.Getenv("DATABASE_URL"); u != "" {
		if p, e := pgxpool.New(ctx, u); e == nil {
			s.db = p
			s.migrate(ctx)
			s.load(ctx)
		}
	}
	return s
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func (s *Server) migrate(ctx context.Context) {
	b, e := os.ReadFile("migrations/001_init.sql")
	if e == nil {
		_, _ = s.db.Exec(ctx, string(b))
	}
	_, _ = s.db.Exec(ctx, "INSERT INTO users(id,login,name,avatar_url) VALUES('00000000-0000-0000-0000-000000000001','octocat','The Octocat','https://github.com/octocat.png') ON CONFLICT DO NOTHING")
	_, _ = s.db.Exec(ctx, "INSERT INTO mode_stats(user_id,mode) VALUES('00000000-0000-0000-0000-000000000001','solo') ON CONFLICT DO NOTHING")
}
func (s *Server) load(ctx context.Context) {
	rows, e := s.db.Query(ctx, "SELECT state FROM games")
	if e == nil {
		defer rows.Close()
		for rows.Next() {
			var b []byte
			if rows.Scan(&b) == nil {
				var g State
				if json.Unmarshal(b, &g) == nil {
					s.games[g.ID] = &g
				}
			}
		}
	}
	var p PublicStats
	if s.db.QueryRow(ctx, "SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM mode_stats WHERE user_id='00000000-0000-0000-0000-000000000001' AND mode='solo'").Scan(&p.Games, &p.Wins, &p.Losses, &p.Rating, &p.Shots, &p.Hits, &p.CurrentStreak, &p.LongestStreak, &p.WinShots) == nil {
		s.stats = decorate(p)
	}
}
func (s *Server) persist(ctx context.Context, g *State) {
	if s.db == nil {
		return
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	b, _ := json.Marshal(g)
	if _, err = tx.Exec(ctx, "INSERT INTO games(id,mode,status,current_turn,winner,player_id,player_start,enemy_start,state,terminal_applied) VALUES($1,'solo',$2,$3,$4,'00000000-0000-0000-0000-000000000001',$5,$6,$7,$8) ON CONFLICT(id) DO UPDATE SET status=$2,current_turn=$3,winner=$4,state=$7,terminal_applied=$8,updated_at=now()", g.ID, g.Status, g.Turn, null(g.Winner), g.PlayerStart, g.EnemyStart, b, g.TerminalApplied); err != nil {
		return
	}
	p := g.Stats
	if _, err = tx.Exec(ctx, "UPDATE mode_stats SET games=$1,wins=$2,losses=$3,rating=$4,shots=$5,hits=$6,current_streak=$7,longest_streak=$8,win_shots=$9 WHERE user_id='00000000-0000-0000-0000-000000000001' AND mode='solo'", p.Games, p.Wins, p.Losses, p.Rating, p.Shots, p.Hits, p.CurrentStreak, p.LongestStreak, p.WinShots); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}
func null(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("POST /v1/dev/session", s.devSession)
	m.HandleFunc("GET /v1/me", s.me)
	m.HandleFunc("GET /v1/me/contributions", s.contributions)
	m.HandleFunc("POST /v1/games/solo", s.create)
	m.HandleFunc("GET /v1/games/{id}", s.get)
	m.HandleFunc("POST /v1/games/{id}/shots", s.shot)
	m.HandleFunc("GET /share/users/{file}", s.svg)
	m.HandleFunc("GET /share/games/{file}", s.png)
	m.HandleFunc("GET /s/{id}", s.share)
	m.HandleFunc("GET /auth/github/start", s.oauthStart)
	return s.cors(m)
}
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.webOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, n int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(n)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) devSession(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"token": "dev-octocat", "user": map[string]string{"login": "octocat", "name": "The Octocat", "avatarUrl": "https://github.com/octocat.png"}})
}
func decorate(p PublicStats) PublicStats {
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
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	write(w, 200, map[string]any{"login": "octocat", "name": "The Octocat", "avatarUrl": "https://github.com/octocat.png", "solo": decorate(s.stats), "pvp": decorate(PublicStats{Rating: 1200})})
}
func cells() []game.Cell {
	start := time.Now().UTC().AddDate(-1, 0, 0)
	for start.Weekday() != time.Sunday {
		start = start.AddDate(0, 0, -1)
	}
	out := make([]game.Cell, 0, 364)
	for i := 0; i < 364; i++ {
		d := start.AddDate(0, 0, i)
		seed := (i*17 + i/7*11) % 23
		count := 0
		if seed > 8 {
			count = (seed*seed)%19 + 1
		}
		level := 0
		if count > 0 {
			level = 1
		}
		if count > 3 {
			level = 2
		}
		if count > 8 {
			level = 3
		}
		if count > 14 {
			level = 4
		}
		out = append(out, game.Cell{Date: d.Format("2006-01-02"), Weekday: int(d.Weekday()), ContributionCount: count, ContributionLevel: level})
	}
	return out
}
func (s *Server) contributions(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"login": "octocat", "days": cells()})
}
func id() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type createReq struct {
	StartDate string      `json:"startDate"`
	Fleet     []game.Ship `json:"fleet"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var q createReq
	if json.NewDecoder(r.Body).Decode(&q) != nil || game.ValidateFleet(q.Fleet) != nil {
		write(w, 422, map[string]any{"error": map[string]string{"code": "invalid_fleet", "message": "Place every ship in bounds without overlap."}})
		return
	}
	days := cells()
	idx := -1
	for i, c := range days {
		if c.Date == q.StartDate {
			idx = i / 7
			break
		}
	}
	if idx < 0 || idx > 42 {
		write(w, 422, map[string]string{"error": "invalid start date"})
		return
	}
	enemyIdx, e := game.OpponentStart(52, idx, game.SecureRand{})
	if e != nil {
		write(w, 500, map[string]string{"error": e.Error()})
		return
	}
	ef, e := game.PlaceFleet(game.SecureRand{})
	if e != nil {
		write(w, 500, map[string]string{"error": e.Error()})
		return
	}
	gid := id()
	g := &State{
		ID: gid, Status: "battle", Turn: "player",
		PlayerBoard: append([]game.Cell(nil), days[idx*7:idx*7+70]...),
		EnemyBoard:  stripDates(days[enemyIdx*7 : enemyIdx*7+70]),
		PlayerFleet: q.Fleet, EnemyFleet: ef,
		PlayerShots: []game.Shot{}, AIShots: []game.Shot{},
		PlayerStart: q.StartDate, EnemyStart: days[enemyIdx*7].Date,
		Stats: decorate(s.stats),
	}
	s.mu.Lock()
	s.games[gid] = g
	s.persist(r.Context(), g)
	s.mu.Unlock()
	write(w, 201, public(g))
}
func stripDates(c []game.Cell) []game.Cell {
	o := append([]game.Cell(nil), c...)
	for i := range o {
		o[i].Date = ""
	}
	return o
}
func public(g *State) map[string]any {
	b, _ := json.Marshal(g)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if g.Status == "complete" {
		m["enemyPeriod"] = map[string]string{"start": g.EnemyStart, "end": dateEnd(g.EnemyStart)}
	}
	return m
}
func dateEnd(v string) string {
	d, _ := time.Parse("2006-01-02", v)
	return d.AddDate(0, 0, 69).Format("2006-01-02")
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.games[r.PathValue("id")]
	if g == nil {
		write(w, 404, map[string]string{"error": "game not found"})
		return
	}
	write(w, 200, public(g))
}
func (s *Server) shot(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.games[r.PathValue("id")]
	if g == nil {
		write(w, 404, map[string]string{"error": "game not found"})
		return
	}
	if g.Status == "complete" {
		write(w, 409, map[string]string{"error": "game is complete"})
		return
	}
	if g.Turn != "player" {
		write(w, 409, map[string]string{"error": "wrong turn"})
		return
	}
	var c game.Coord
	if json.NewDecoder(r.Body).Decode(&c) != nil {
		write(w, 400, map[string]string{"error": "invalid coordinate"})
		return
	}
	ev, n, e := game.ResolveShot(g.EnemyFleet, g.PlayerShots, c)
	if e != nil {
		write(w, 409, map[string]string{"error": e.Error()})
		return
	}
	g.EnemyFleet = n
	g.PlayerShots = append(g.PlayerShots, ev)
	events := []game.Shot{ev}
	if game.AllSunk(g.EnemyFleet) {
		s.finish(g, "player")
	} else {
		g.Turn = "ai"
		target, _ := game.NextAITarget(g.AIShots, game.SecureRand{})
		a, pf, _ := game.ResolveShot(g.PlayerFleet, g.AIShots, target)
		g.PlayerFleet = pf
		g.AIShots = append(g.AIShots, a)
		events = append(events, a)
		if game.AllSunk(g.PlayerFleet) {
			s.finish(g, "ai")
		} else {
			g.Turn = "player"
		}
	}
	s.persist(r.Context(), g)
	write(w, 200, map[string]any{"game": public(g), "events": events})
}
func (s *Server) finish(g *State, winner string) {
	if g.TerminalApplied {
		return
	}
	g.Status = "complete"
	g.Turn = "complete"
	g.Winner = winner
	g.TerminalApplied = true
	won := winner == "player"
	old := s.stats.Rating
	gs := game.Stats{Games: s.stats.Games, Wins: s.stats.Wins, Losses: s.stats.Losses, Rating: s.stats.Rating, Shots: s.stats.Shots, Hits: s.stats.Hits, CurrentStreak: s.stats.CurrentStreak, LongestStreak: s.stats.LongestStreak, WinShots: s.stats.WinShots}
	gs = game.UpdateStats(gs, won, len(g.PlayerShots), countHits(g.PlayerShots))
	s.stats = decorate(PublicStats{Games: gs.Games, Wins: gs.Wins, Losses: gs.Losses, Rating: gs.Rating, Shots: gs.Shots, Hits: gs.Hits, CurrentStreak: gs.CurrentStreak, LongestStreak: gs.LongestStreak, WinShots: gs.WinShots})
	g.Stats = s.stats
	g.RatingDelta = s.stats.Rating - old
	g.ShareID = id()[:12]
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
func (s *Server) svg(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.PathValue("file"), ".svg") {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	p := decorate(s.stats)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "image/svg+xml")
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="120" role="img" aria-label="GitHarbour stats"><rect width="100%%" height="100%%" rx="8" fill="#0d1117"/><text x="24" y="36" fill="#f0f6fc" font-family="system-ui" font-size="20" font-weight="600">GitHarbour · %s</text><text x="24" y="70" fill="#8c959f" font-family="system-ui" font-size="15">Solo rating %d · %d wins · %.0f%% accuracy</text><text x="24" y="98" fill="#3fb950" font-family="system-ui" font-size="13">Your GitHub history is a battlefield.</text></svg>`, p.Rank, p.Rating, p.Wins, p.Accuracy)
}
func (s *Server) png(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.PathValue("file"), ".png") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	// Replaceable placeholder renderer; production will draw the documented 1200×630 card.
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M/wHwAFAgIAn2kOCQAAAABJRU5ErkJggg==")
	_, _ = w.Write(b)
}
func (s *Server) share(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html><head><meta property="og:title" content="GitHarbour battle result"><meta property="og:description" content="A developer battled their own GitHub history."><meta property="og:image" content="%s/share/games/%s.png"><meta name="twitter:card" content="summary_large_image"><title>GitHarbour result</title></head><body><h1>GitHarbour</h1><p>Your GitHub history is a battlefield.</p></body></html>`, env("PUBLIC_API_URL", "http://localhost:8080"), sid)
}
func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	client := os.Getenv("GITHUB_CLIENT_ID")
	if client == "" {
		write(w, 503, map[string]string{"error": "GitHub OAuth is not configured; use development session"})
		return
	}
	state := id()
	scope := "read:user"
	u := "https://github.com/login/oauth/authorize?client_id=" + client + "&scope=" + scope + "&state=" + state
	http.Redirect(w, r, u, 302)
}
func Int(v string) int { n, _ := strconv.Atoi(strings.TrimSpace(v)); return n }
