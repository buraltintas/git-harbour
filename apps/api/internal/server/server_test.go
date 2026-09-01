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

func TestHealthEndpoints(t *testing.T) {
	s, _ := testServer(t)
	for _, path := range []string{"/health", "/healthz"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, w.Code, w.Body.String())
		}
	}
}

type fixedRand int

func (f fixedRand) Intn(n int) (int, error) { return int(f) % n, nil }

func reciprocalState(playerTargets, enemyTargets []int) *State {
	start := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	makeBoard := func(targets []int) []game.Cell {
		board := make([]game.Cell, game.BoardCells)
		for i := range board {
			board[i] = game.Cell{Date: start.AddDate(0, 0, i).Format("2006-01-02"), Weekday: i % 7}
		}
		for _, i := range targets {
			board[i].ContributionCount, board[i].ContributionLevel = i+1, 1
		}
		return board
	}
	player, enemy := makeBoard(playerTargets), makeBoard(enemyTargets)
	return &State{ID: "reciprocal", Ruleset: "contribution_targets_v2", Status: "battle", Turn: "player", PlayerBoard: player, EnemyBoard: enemy, PlayerStart: player[0].Date, EnemyStart: enemy[0].Date, PlayerTargetCount: game.TargetCount(player), EnemyTargetCount: game.TargetCount(enemy), Stats: decorate(PublicStats{Rating: 1200})}
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
	g := &State{ID: uuid(), Ruleset: "contribution_targets_v2", Status: "battle", Turn: "player", PlayerBoard: board, EnemyBoard: append([]game.Cell(nil), board...), PlayerStart: board[0].Date, EnemyStart: board[0].Date, PlayerTargetCount: game.TargetCount(board), EnemyTargetCount: game.TargetCount(board)}
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
	all := mockCells(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	player, enemy := all[:70], all[70:140]
	g := &State{ID: "game", Ruleset: "contribution_targets_v2", Status: "battle", Turn: "player", PlayerBoard: player, EnemyBoard: enemy, PlayerStart: player[0].Date, EnemyStart: enemy[0].Date, PlayerTargetCount: game.TargetCount(player), EnemyTargetCount: game.TargetCount(enemy)}
	b, _ := json.Marshal(g)
	if !strings.Contains(string(b), enemy[0].Date) || !strings.Contains(string(b), "contributionCount") {
		t.Fatal("stored state lost the frozen contribution snapshot")
	}
	projection := publicGame(g)
	public, _ := json.Marshal(projection)
	if strings.Contains(string(public), enemy[0].Date) || strings.Contains(string(public), `"enemyPeriod"`) {
		t.Fatal("public projection exposed hidden state")
	}
	for _, cell := range projection["enemyCells"].([]map[string]any) {
		if cell["state"] == "unknown" && len(cell) != 3 {
			t.Fatal("unknown enemy cell leaked hidden fields", cell)
		}
	}
	if first := projection["playerCells"].([]map[string]any)[0]; first["date"] == nil || first["contributionCount"] == nil {
		t.Fatal("owner could not see selected player harbour", first)
	}
	target := game.Coord{}
	for i, cell := range enemy {
		if cell.ContributionCount > 0 {
			target = game.Coord{X: i / 7, Y: i % 7}
			break
		}
	}
	shot, _, _ := game.ResolveTargetShot(enemy, nil, target)
	g.PlayerTargetShots = []game.TargetShot{shot}
	projection = publicGame(g)
	public, _ = json.Marshal(projection)
	if !strings.Contains(string(public), "contributionCount") || strings.Contains(string(public), enemy[0].Date) {
		t.Fatal("hit reveal should expose its intensity but not dates", string(public))
	}
	revealed := projection["enemyCells"].([]map[string]any)[target.X*game.Height+target.Y]
	if revealed["date"] != nil || revealed["weekday"] != nil || revealed["contributionCount"] == nil || revealed["contributionLevel"] == nil {
		t.Fatal("active hit projection has unsafe or missing fields", revealed)
	}
}

func TestSoloContributionFleetCreateDeployAndAct(t *testing.T) {
	s, repo := testServer(t)
	token := devToken(t, s)
	days, _ := repo.Contributions(context.Background(), repo.sessions[hashKey(digest(token))].UserID)
	created := httptest.NewRecorder()
	s.Handler().ServeHTTP(created, authed("POST", "/v1/games/solo", token, fmt.Sprintf(`{"playerStart":%q}`, days[0].Date)))
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &response)
	if response.ID == "" || response.Status != "deployment" || !strings.Contains(created.Body.String(), `"playerSummary"`) || strings.Contains(created.Body.String(), `"enemyPeriod"`) || strings.Contains(created.Body.String(), frozenEnemyDateMarker(days)) {
		t.Fatal("active game leaked hidden board", created.Body.String())
	}
	repo.mu.Lock()
	frozen := cloneState(repo.games[response.ID])
	repo.mu.Unlock()
	if len(frozen.PlayerBoard) != 70 || len(frozen.EnemyBoard) != 70 || frozen.PlayerStart == frozen.EnemyStart {
		t.Fatal("game must freeze two different 70-cell boards")
	}
	if len(frozen.EnemyDeployment) != game.FleetCapacity(game.TargetCount(frozen.EnemyBoard)) || frozen.Turn != "setup" {
		t.Fatal("computer deployment or setup turn is invalid")
	}
	choices := deploymentChoices(frozen.PlayerBoard)
	body, _ := json.Marshal(map[string]any{"units": choices})
	deployed := httptest.NewRecorder()
	s.Handler().ServeHTTP(deployed, authed("POST", "/v1/games/"+response.ID+"/deployment", token, string(body)))
	if deployed.Code != 200 || !strings.Contains(deployed.Body.String(), `"status":"battle"`) {
		t.Fatal(deployed.Code, deployed.Body.String())
	}
	repo.mu.Lock()
	ready := cloneState(repo.games[response.ID])
	repo.mu.Unlock()
	actionBody, _ := json.Marshal(map[string]any{"attacker": ready.PlayerDeployment[0].Coord, "target": game.Coord{X: 0, Y: 0}})
	action := httptest.NewRecorder()
	s.Handler().ServeHTTP(action, authed("POST", "/v1/games/"+response.ID+"/actions", token, string(actionBody)))
	if action.Code != 200 || !strings.Contains(action.Body.String(), `"events"`) {
		t.Fatal(action.Code, action.Body.String())
	}
}

func frozenEnemyDateMarker(days []game.Cell) string {
	if len(days) > 70 {
		return days[70].Date
	}
	return "never"
}
func deploymentChoices(board []game.Cell) []game.DeploymentChoice {
	capacity := game.FleetCapacity(game.TargetCount(board))
	out := []game.DeploymentChoice{}
	for i, cell := range board {
		if cell.ContributionCount > 0 && len(out) < capacity {
			out = append(out, game.DeploymentChoice{Coord: game.Coord{X: i / 7, Y: i % 7}, Kind: "contribution"})
		}
	}
	for i, cell := range board {
		if cell.ContributionCount == 0 && len(out) < capacity {
			out = append(out, game.DeploymentChoice{Coord: game.Coord{X: i / 7, Y: i % 7}, Kind: "reserve"})
		}
	}
	return out
}

func TestContributionFleetVictoryDefeatAndExposure(t *testing.T) {
	board := mockCells(time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC))[:70]
	base := func() *State {
		return &State{Ruleset: game.ContributionFleetRuleset, Status: "battle", Turn: "player", PlayerBoard: board, EnemyBoard: append([]game.Cell(nil), board...), PlayerDeployment: []game.FleetUnit{{Coord: game.Coord{X: 0, Y: 0}, Kind: "reserve", Power: 1, Alive: true}}, EnemyDeployment: []game.FleetUnit{{Coord: game.Coord{X: 1, Y: 1}, Kind: "reserve", Power: 1, Alive: true}}}
	}
	win := base()
	events, stats, err := resolveFleetTurn(win, decorate(PublicStats{Rating: 1200}), game.Coord{X: 0, Y: 0}, game.Coord{X: 1, Y: 1}, fixedRand(0))
	if err != nil || len(events) != 1 || win.Winner != "player" || stats.Wins != 1 || !win.PlayerDeployment[0].Exposed || win.EnemyDeployment[0].Alive {
		t.Fatal(events, stats, win, err)
	}
	if _, again, err := resolveFleetTurn(win, stats, game.Coord{X: 0, Y: 0}, game.Coord{X: 1, Y: 1}, fixedRand(0)); err != ErrGameComplete || again.Games != 1 {
		t.Fatal("terminal v3 stats applied twice", again, err)
	}
	loss := base()
	events, stats, err = resolveFleetTurn(loss, decorate(PublicStats{Rating: 1200}), game.Coord{X: 0, Y: 0}, game.Coord{X: 2, Y: 2}, fixedRand(0))
	if err != nil || len(events) != 2 || loss.Winner != "ai" || stats.Losses != 1 || loss.PlayerDeployment[0].Alive {
		t.Fatal(events, stats, loss, err)
	}
}

func TestContributionFleetProjectionHidesDeploymentUntilExposure(t *testing.T) {
	board := mockCells(time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC))[:70]
	g := &State{Ruleset: game.ContributionFleetRuleset, Status: "battle", Turn: "player", PlayerBoard: board, EnemyBoard: append([]game.Cell(nil), board...), PlayerDeployment: []game.FleetUnit{{Coord: game.Coord{X: 0, Y: 0}, Kind: "reserve", Power: 1, Alive: true}}, EnemyDeployment: []game.FleetUnit{{Coord: game.Coord{X: 1, Y: 1}, Kind: "contribution", ContributionCount: 99, Power: game.DayPower(99), Level: 3, Alive: true}}, PlayerStart: board[0].Date, EnemyStart: board[0].Date}
	active := publicGame(g)
	if active["enemyPeriod"] != nil {
		t.Fatal("enemy period leaked")
	}
	for _, cell := range active["enemyCells"].([]map[string]any) {
		if cell["unitKind"] != nil || cell["unitPower"] != nil || cell["date"] != nil {
			t.Fatal("hidden deployment leaked", cell)
		}
	}
	g.EnemyDeployment[0].Exposed = true
	projection := publicGame(g)
	cell := projection["enemyCells"].([]map[string]any)[8]
	if cell["state"] != "exposed" || cell["combatLevel"] != 3 || cell["unitPower"] != nil {
		t.Fatal("exposure projection wrong", cell)
	}
	g.Status = "complete"
	encoded, _ := json.Marshal(publicGame(g))
	if !strings.Contains(string(encoded), `"unitPower"`) || !strings.Contains(string(encoded), `"enemyPeriod"`) {
		t.Fatal("terminal reveal incomplete", string(encoded))
	}
}

func TestFleetSnapshotSurvivesContributionRefreshAndLegacyCompletionReads(t *testing.T) {
	repo := NewMemoryRepository()
	original := mockCells(time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC))
	u, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 501, Login: "frozen"}, original)
	g := &State{ID: uuid(), Ruleset: game.ContributionFleetRuleset, Status: "deployment", Turn: "setup", PlayerBoard: append([]game.Cell(nil), original[:70]...), EnemyBoard: append([]game.Cell(nil), original[70:140]...)}
	_ = repo.CreateGame(context.Background(), u.ID, g)
	refreshed := mockCells(time.Date(2027, 1, 3, 0, 0, 0, 0, time.UTC))
	_, _ = repo.UpsertGitHubUser(context.Background(), User{GitHubID: 501, Login: "frozen"}, refreshed)
	stored, err := repo.Game(context.Background(), u.ID, g.ID)
	if err != nil || stored.PlayerBoard[0].Date != original[0].Date {
		t.Fatal("frozen board changed after refresh", err)
	}
	legacy := reciprocalState([]int{0}, []int{69})
	legacy.ID = uuid()
	legacy.Status, legacy.Turn = "complete", "complete"
	_ = repo.CreateGame(context.Background(), u.ID, legacy)
	if _, err = repo.Game(context.Background(), u.ID, legacy.ID); err != nil {
		t.Fatal("completed v2 game must remain readable", err)
	}
}

func TestReciprocalTurnVictoryAndDefeat(t *testing.T) {
	p := decorate(PublicStats{Rating: 1200})
	g := reciprocalState([]int{69}, []int{0, 1})
	events, updated, err := resolveTargetTurn(g, p, game.Coord{X: 0, Y: 0}, fixedRand(0))
	if err != nil || len(events) != 2 || events[0].Actor != "player" || events[1].Actor != "ai" || g.Turn != "player" || len(g.AITargetShots) != 1 {
		t.Fatal(events, updated, g.Turn, err)
	}
	if _, _, err = resolveTargetTurn(g, p, game.Coord{X: 0, Y: 0}, fixedRand(0)); err == nil {
		t.Fatal("duplicate player shot accepted")
	}
	events, updated, err = resolveTargetTurn(g, p, game.Coord{X: 0, Y: 1}, fixedRand(0))
	if err != nil || len(events) != 1 || g.Winner != "player" || updated.Wins != 1 || updated.Losses != 0 || updated.Rating <= 1200 {
		t.Fatal(events, updated, g.Winner, err)
	}
	if _, again, err := resolveTargetTurn(g, updated, game.Coord{X: 1, Y: 1}, fixedRand(0)); err != ErrGameComplete || again.Games != 1 {
		t.Fatal("terminal state was not stable", again, err)
	}

	loss := reciprocalState([]int{0}, []int{69})
	loser := decorate(PublicStats{Rating: 1200, CurrentStreak: 3, LongestStreak: 3})
	events, loser, err = resolveTargetTurn(loss, loser, game.Coord{X: 0, Y: 0}, fixedRand(0))
	if err != nil || len(events) != 2 || loss.Winner != "ai" || loser.Wins != 0 || loser.Losses != 1 || loser.CurrentStreak != 0 || loser.Rating >= 1200 || loser.Shots != 1 || loser.Hits != 0 {
		t.Fatal(events, loser, loss.Winner, err)
	}
}

func TestMemoryTurnIsAtomicWhenAISelectionFails(t *testing.T) {
	repo := NewMemoryRepository()
	u, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 88, Login: "atomic"}, mockCells(time.Now()))
	g := reciprocalState([]int{69}, []int{0, 1})
	g.ID = uuid()
	for x := 0; x < game.Width; x++ {
		for y := 0; y < game.Height; y++ {
			cell := g.PlayerBoard[x*game.Height+y]
			shot := game.TargetShot{Coord: game.Coord{X: x, Y: y}, Result: "miss"}
			if cell.ContributionCount > 0 {
				shot.Result, shot.ContributionCount, shot.ContributionLevel = "hit", cell.ContributionCount, cell.ContributionLevel
			}
			g.AITargetShots = append(g.AITargetShots, shot)
		}
	}
	_ = repo.CreateGame(context.Background(), u.ID, g)
	if _, _, err := repo.Shoot(context.Background(), u.ID, g.ID, game.Coord{X: 0, Y: 0}); err == nil {
		t.Fatal("expected exhausted AI target error")
	}
	repo.mu.Lock()
	stored := cloneState(repo.games[g.ID])
	repo.mu.Unlock()
	if len(stored.PlayerTargetShots) != 0 || stored.Turn != "player" {
		t.Fatal("failed transition partially mutated memory state", stored.PlayerTargetShots, stored.Turn)
	}
}

func TestTerminalProjectionRevealsEnemyPeriod(t *testing.T) {
	g := reciprocalState([]int{0}, []int{69})
	g.Status, g.Turn, g.Winner = "complete", "complete", "ai"
	public, _ := json.Marshal(publicGame(g))
	if !strings.Contains(string(public), `"enemyPeriod"`) || !strings.Contains(string(public), g.EnemyStart) || !strings.Contains(string(public), `"date"`) {
		t.Fatal(string(public))
	}
	if !strings.Contains(string(public), `"state":"target"`) {
		t.Fatal("unhit enemy targets should be revealed without being mislabeled as player hits", string(public))
	}
}

func TestReciprocalSoloShareUsesResultAndBothPeriods(t *testing.T) {
	s, repo := testServer(t)
	u, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 71, Login: "alice"}, mockCells(s.now()))
	g := reciprocalState([]int{0}, []int{69})
	g.ID, g.Status, g.Turn, g.Winner, g.ShareID, g.RatingDelta = uuid(), "complete", "complete", "player", "share-one", 16
	g.PlayerTargetShots = []game.TargetShot{{Coord: game.Coord{X: 9, Y: 6}, Result: "hit", ContributionCount: 70, ContributionLevel: 1}}
	_ = repo.CreateGame(context.Background(), u.ID, g)
	repo.shares[g.ShareID] = g.ID
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/s/share-one", nil)
	r.SetPathValue("id", "share-one")
	s.shareHTML(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Victory") || !strings.Contains(w.Body.String(), g.PlayerStart) || !strings.Contains(w.Body.String(), g.EnemyStart) {
		t.Fatal(w.Code, w.Body.String())
	}
	if img, err := renderSoloShareCard(g, u); err != nil || img.Bounds().Dx() != 1200 {
		t.Fatal("reciprocal share card failed", err)
	}
}
func TestContributionFleetShareUsesFleetSemantics(t *testing.T) {
	s, repo := testServer(t)
	u, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 72, Login: "fleet-user"}, mockCells(s.now()))
	board := mockCells(s.now())[:70]
	g := &State{ID: uuid(), Ruleset: game.ContributionFleetRuleset, Status: "complete", Turn: "complete", Winner: "player", PlayerBoard: board, EnemyBoard: append([]game.Cell(nil), board...), PlayerDeployment: []game.FleetUnit{{Coord: game.Coord{X: 0, Y: 0}, Kind: "reserve", Power: 1, Alive: true}}, EnemyDeployment: []game.FleetUnit{{Coord: game.Coord{X: 1, Y: 1}, Kind: "reserve", Power: 1, Alive: false, Exposed: true}}, FleetActions: []game.FleetAction{{Actor: "player", Attacker: game.Coord{X: 0, Y: 0}, Target: game.Coord{X: 1, Y: 1}, Result: "clash", AttackerWon: true, Probability: .5, Roll: .2, AttackerPower: 1, DefenderPower: 1}}, PlayerStart: board[0].Date, EnemyStart: board[0].Date, ShareID: "fleet-share", RatingDelta: 16}
	_ = repo.CreateGame(context.Background(), u.ID, g)
	repo.shares[g.ShareID] = g.ID
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/s/fleet-share", nil)
	r.SetPathValue("id", "fleet-share")
	s.shareHTML(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "1 actions") || !strings.Contains(w.Body.String(), "1 clashes") || strings.Contains(w.Body.String(), "targets") {
		t.Fatal(w.Code, w.Body.String())
	}
	if img, err := renderSoloShareCard(g, u); err != nil || img.Bounds().Dx() != 1200 {
		t.Fatal("fleet share card failed", err)
	}
}

func TestActiveFleetProjectionExplainsRecentActionsWithoutSecretRolls(t *testing.T) {
	board := mockCells(time.Now().UTC())[:70]
	g := &State{Ruleset: game.ContributionFleetRuleset, Status: "battle", Turn: "player", PlayerBoard: board, EnemyBoard: append([]game.Cell(nil), board...), PlayerDeployment: []game.FleetUnit{{Coord: game.Coord{X: 0, Y: 0}, Kind: "reserve", Power: 1, Alive: true, Exposed: true}}, EnemyDeployment: []game.FleetUnit{{Coord: game.Coord{X: 1, Y: 1}, Kind: "reserve", Power: 1, Alive: false, Exposed: true}}, FleetActions: []game.FleetAction{{Actor: "ai", Attacker: game.Coord{X: 1, Y: 1}, Target: game.Coord{X: 0, Y: 0}, Result: "clash", AttackerWon: false, Probability: .5, Roll: .8, AttackerPower: 1, DefenderPower: 1}}, PlayerStart: board[0].Date, EnemyStart: board[0].Date}
	b, err := json.Marshal(publicGame(g))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, `"recentActions":[{"actor":"ai"`) || !strings.Contains(text, `"result":"clash"`) {
		t.Fatal("recent action explanation missing", text)
	}
	if strings.Contains(text, `"probability"`) || strings.Contains(text, `"roll"`) || strings.Contains(text, `"attackerPower"`) {
		t.Fatal("active projection leaked authoritative combat secrets", text)
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
