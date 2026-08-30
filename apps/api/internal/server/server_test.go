package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func testServer(t *testing.T) (*Server, *MemoryRepository) {
	t.Helper()
	repo := NewMemoryRepository()
	cfg := Config{AppEnv: "test", DevAuth: true, WebOrigins: "http://localhost:5173", PublicAPIURL: "https://api.example", WebAppURL: "https://web.example/#", GitHubClientID: "client", GitHubClientSecret: "secret", GitHubCallback: "https://api.example/auth/github/callback", GitHubWebCallback: "https://web.example/#/auth/callback"}
	s, err := NewWithConfig(context.Background(), cfg, repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	return s, repo
}
func devToken(t *testing.T, s *Server) string {
	t.Helper()
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/v1/dev/session", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.Token
}
func authed(method, path, token, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestOAuthStateSecureSingleUseAndExpiry(t *testing.T) {
	s, repo := testServer(t)
	a := httptest.NewRecorder()
	s.Handler().ServeHTTP(a, httptest.NewRequest("GET", "/auth/github/start", nil))
	b := httptest.NewRecorder()
	s.Handler().ServeHTTP(b, httptest.NewRequest("GET", "/auth/github/start", nil))
	if a.Code != 302 || b.Code != 302 || a.Header().Get("Location") == b.Header().Get("Location") {
		t.Fatal("state is not random")
	}
	if strings.Contains(a.Header().Get("Location"), "scope=") {
		t.Fatal("OAuth must request no explicit scope")
	}
	state := randomSecret(24)
	hash := digest(state)
	_ = repo.PutOAuthState(context.Background(), hash, s.now().Add(time.Minute))
	if err := repo.ConsumeOAuthState(context.Background(), hash, s.now()); err != nil {
		t.Fatal(err)
	}
	if err := repo.ConsumeOAuthState(context.Background(), hash, s.now()); err == nil {
		t.Fatal("replayed state accepted")
	}
	expired := digest("expired")
	_ = repo.PutOAuthState(context.Background(), expired, s.now().Add(-time.Second))
	if err := repo.ConsumeOAuthState(context.Background(), expired, s.now()); err == nil {
		t.Fatal("expired state accepted")
	}
}
func TestExchangeAndSessionLifecycle(t *testing.T) {
	s, repo := testServer(t)
	u, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 1, Login: "alice", Name: "Alice"}, mockCells(s.now()))
	code := "one-time"
	_ = repo.PutExchangeCode(context.Background(), digest(code), u.ID, s.now().Add(time.Minute))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/auth/exchange", strings.NewReader(`{"code":"one-time"}`)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	replay := httptest.NewRecorder()
	s.Handler().ServeHTTP(replay, httptest.NewRequest("POST", "/auth/exchange", strings.NewReader(`{"code":"one-time"}`)))
	if replay.Code != 401 {
		t.Fatal("exchange replay accepted")
	}
	me := httptest.NewRecorder()
	s.Handler().ServeHTTP(me, authed("GET", "/v1/me", out.Token, ""))
	if me.Code != 200 || !strings.Contains(me.Body.String(), "alice") {
		t.Fatal(me.Code, me.Body.String())
	}
	logout := httptest.NewRecorder()
	s.Handler().ServeHTTP(logout, authed("POST", "/auth/logout", out.Token, ""))
	if logout.Code != 204 {
		t.Fatal(logout.Code)
	}
	revoked := httptest.NewRecorder()
	s.Handler().ServeHTTP(revoked, authed("GET", "/v1/me", out.Token, ""))
	if revoked.Code != 401 {
		t.Fatal("revoked session accepted")
	}
}
func TestExpiredExchangeAndInvalidSession(t *testing.T) {
	s, repo := testServer(t)
	u, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 2, Login: "bob"}, mockCells(s.now()))
	_ = repo.PutExchangeCode(context.Background(), digest("old"), u.ID, s.now().Add(-time.Second))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/auth/exchange", strings.NewReader(`{"code":"old"}`)))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, authed("GET", "/v1/me", "invalid", ""))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/me", nil))
	if w.Code != 401 {
		t.Fatal("private endpoint allowed anonymous access")
	}
}
func TestGitHubStableIdentityAndContributionMapping(t *testing.T) {
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"access_token": "github-secret"})
	}))
	defer token.Close()
	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer github-secret" {
			t.Error("missing token")
		}
		days := make([]map[string]any, 70)
		levels := []string{"NONE", "FIRST_QUARTILE", "SECOND_QUARTILE", "THIRD_QUARTILE", "FOURTH_QUARTILE"}
		for i := range days {
			days[i] = map[string]any{"date": time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02"), "weekday": i % 7, "contributionCount": i, "contributionLevel": levels[i%5]}
		}
		b, _ := json.Marshal(days)
		fmt.Fprintf(w, `{"data":{"viewer":{"databaseId":42,"login":"Alice","name":"Alice A","avatarUrl":"https://example/a.png","contributionsCollection":{"contributionCalendar":{"weeks":[{"contributionDays":%s}]}}}}}`, b)
	}))
	defer graph.Close()
	cfg := Config{GitHubClientID: "c", GitHubClientSecret: "s", GitHubCallback: "https://cb", GitHubTokenURL: token.URL, GitHubGraphQLURL: graph.URL}
	u, days, err := NewHTTPGitHubClient(cfg).Authenticate(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	if u.GitHubID != 42 || u.Login != "Alice" || days[4].ContributionLevel != 4 {
		t.Fatal(u, days[4])
	}
	repo := NewMemoryRepository()
	first, _ := repo.UpsertGitHubUser(context.Background(), u, days)
	u.Login = "AliceRenamed"
	second, _ := repo.UpsertGitHubUser(context.Background(), u, days)
	if first.ID != second.ID || second.Login != "AliceRenamed" {
		t.Fatal("GitHub ID did not preserve identity")
	}
}
func TestAuthorizationBetweenUsers(t *testing.T) {
	s, repo := testServer(t)
	a, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 10, Login: "a"}, mockCells(s.now()))
	b, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 11, Login: "b"}, mockCells(s.now()))
	board := mockCells(s.now())[:70]
	g := &State{ID: uuid(), Ruleset: "contribution_targets_v2", Status: "battle", Turn: "player", Board: board, PeriodStart: board[0].Date, TargetCount: game.TargetCount(board)}
	_ = repo.CreateGame(context.Background(), a.ID, g)
	if _, err := repo.Game(context.Background(), b.ID, g.ID); err != ErrNotFound {
		t.Fatal("user B read user A game")
	}
	if _, _, err := repo.Shoot(context.Background(), b.ID, g.ID, game.Coord{}); err != ErrNotFound {
		t.Fatal("user B shot user A game")
	}
	sa, _ := repo.Stats(context.Background(), a.ID, "solo")
	sb, _ := repo.Stats(context.Background(), b.ID, "solo")
	if sa.Rating != sb.Rating {
		t.Fatal("initial independent stats differ")
	}
}
func TestPublicGameHidesPersistedEnemyState(t *testing.T) {
	board := mockCells(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))[:70]
	g := &State{ID: "game", Ruleset: "contribution_targets_v2", Status: "battle", Turn: "player", Board: board, PeriodStart: board[0].Date, TargetCount: game.TargetCount(board)}
	b, _ := json.Marshal(g)
	if !strings.Contains(string(b), board[0].Date) || !strings.Contains(string(b), "contributionCount") {
		t.Fatal("stored state lost the frozen contribution snapshot")
	}
	public, _ := json.Marshal(publicGame(g))
	if strings.Contains(string(public), "date") || strings.Contains(string(public), "contributionCount") || strings.Contains(string(public), "contributionLevel") || strings.Contains(string(public), "periodStart") {
		t.Fatal("public projection exposed hidden state")
	}
	target := game.Coord{}
	for i, cell := range board {
		if cell.ContributionCount > 0 {
			target = game.Coord{X: i / 7, Y: i % 7}
			break
		}
	}
	shot, _, _ := game.ResolveTargetShot(board, nil, target)
	g.Shots = []game.TargetShot{shot}
	public, _ = json.Marshal(publicGame(g))
	if !strings.Contains(string(public), "contributionCount") || strings.Contains(string(public), board[0].Date) {
		t.Fatal("hit reveal should expose its intensity but not dates", string(public))
	}
}

func TestSoloContributionTargetFlowAndStatsOnce(t *testing.T) {
	s, repo := testServer(t)
	token := devToken(t, s)
	created := httptest.NewRecorder()
	s.Handler().ServeHTTP(created, authed("POST", "/v1/games/solo", token, `{}`))
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var response struct {
		ID          string `json:"id"`
		TargetCount int    `json:"targetCount"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &response)
	if response.ID == "" || response.TargetCount == 0 || strings.Contains(created.Body.String(), "date") || strings.Contains(created.Body.String(), "contributionCount") {
		t.Fatal("active game leaked hidden board", created.Body.String())
	}
	repo.mu.Lock()
	frozen := cloneState(repo.games[response.ID])
	repo.mu.Unlock()
	if len(frozen.Board) != 70 {
		t.Fatal("frozen board must contain exactly 70 cells")
	}
	var finalBody string
	for i, cell := range frozen.Board {
		if cell.ContributionCount == 0 {
			continue
		}
		shot := httptest.NewRecorder()
		body := fmt.Sprintf(`{"x":%d,"y":%d}`, i/7, i%7)
		s.Handler().ServeHTTP(shot, authed("POST", "/v1/games/"+response.ID+"/shots", token, body))
		if shot.Code != 200 {
			t.Fatal(shot.Code, shot.Body.String())
		}
		finalBody = shot.Body.String()
	}
	if !strings.Contains(finalBody, `"status":"complete"`) || !strings.Contains(finalBody, `"period"`) || !strings.Contains(finalBody, frozen.Board[0].Date) {
		t.Fatal("completed game did not reveal the frozen period", finalBody)
	}
	if strings.Contains(finalBody, `"misses":1`) {
		t.Fatal("empty cells should not be required for completion")
	}
	stats, _ := repo.Stats(context.Background(), repo.owners[response.ID], "solo")
	if stats.Games != 1 || stats.Wins != 1 || stats.Hits != response.TargetCount {
		t.Fatal("terminal stats were not applied exactly once", stats)
	}
	more := httptest.NewRecorder()
	s.Handler().ServeHTTP(more, authed("POST", "/v1/games/"+response.ID+"/shots", token, `{"x":9,"y":6}`))
	if more.Code != 409 || !strings.Contains(more.Body.String(), "game_complete") {
		t.Fatal("completed game accepted another shot", more.Code, more.Body.String())
	}
}
func TestPublicProfileWidgetAndEscaping(t *testing.T) {
	s, repo := testServer(t)
	_, _ = repo.UpsertGitHubUser(context.Background(), User{GitHubID: 99, Login: "<alice>", Name: `A & "B"`, AvatarURL: "https://example/a.png"}, mockCells(s.now()))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/widgets/x.svg?theme=light", nil)
	r.SetPathValue("file", "<alice>.svg")
	s.widget(w, r)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/svg+xml; charset=utf-8" || strings.Contains(w.Body.String(), "<alice>") || !strings.Contains(w.Body.String(), "#ffffff") {
		t.Fatal(w.Code, w.Body.String())
	}
	h := httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/u/x", nil)
	r.SetPathValue("login", "<alice>")
	s.publicUserHTML(h, r)
	if h.Code != 200 || strings.Contains(h.Body.String(), `A & "B"`) {
		t.Fatal("HTML was not escaped")
	}
	missing := httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/u/nope", nil)
	r.SetPathValue("login", "nope")
	s.publicUserHTML(missing, r)
	if missing.Code != 404 {
		t.Fatal(missing.Code)
	}
}
func TestProductionRequiresDatabase(t *testing.T) {
	_, err := NewWithConfig(context.Background(), Config{AppEnv: "production"}, nil, nil)
	if err == nil {
		t.Fatal("production accepted missing database")
	}
}
func TestCORSExactOrigins(t *testing.T) {
	s, _ := testServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/v1/me", nil)
	r.Header.Set("Origin", "https://evil.example")
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unexpected CORS")
	}
}
