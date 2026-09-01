package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func pvpFleet() []game.Ship {
	return []game.Ship{{Kind: "Carrier", Cells: []game.Coord{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}, {X: 4, Y: 0}}}, {Kind: "Battleship", Cells: []game.Coord{{X: 0, Y: 1}, {X: 1, Y: 1}, {X: 2, Y: 1}, {X: 3, Y: 1}}}, {Kind: "Cruiser", Cells: []game.Coord{{X: 0, Y: 2}, {X: 1, Y: 2}, {X: 2, Y: 2}}}, {Kind: "Submarine", Cells: []game.Coord{{X: 0, Y: 3}, {X: 1, Y: 3}, {X: 2, Y: 3}}}, {Kind: "Destroyer", Cells: []game.Coord{{X: 0, Y: 4}, {X: 1, Y: 4}}}}
}

func newMemoryBattle(t *testing.T) (*MemoryRepository, User, User, *PVPGame) {
	t.Helper()
	m := NewMemoryRepository()
	ctx := context.Background()
	days := mockCells(time.Now())
	a, err := m.UpsertGitHubUser(ctx, User{GitHubID: 11, Login: "alice"}, days)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.UpsertGitHubUser(ctx, User{GitHubID: 12, Login: "bob"}, days)
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.CreateChallenge(ctx, a.ID, "battle-code", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.AcceptChallenge(ctx, b.ID, c.Code, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err = m.ReadyChallenge(ctx, a.ID, c.Code, days[0].Date, days[:70], pvpFleet(), time.Now()); err != nil {
		t.Fatal(err)
	}
	_, g, err := m.ReadyChallenge(ctx, b.ID, c.Code, days[0].Date, days[:70], pvpFleet(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return m, a, b, g
}

func TestMemoryPVPFullLifecycleStatsHistoryShareAndRematch(t *testing.T) {
	m, a, b, g := newMemoryBattle(t)
	ctx := context.Background()
	winner, loser := a, b
	if g.CurrentTurn == b.ID {
		winner, loser = b, a
	}

	targets := []game.Coord{}
	for _, ship := range pvpFleet() {
		targets = append(targets, ship.Cells...)
	}
	misses := []game.Coord{{X: 9, Y: 6}, {X: 8, Y: 6}, {X: 7, Y: 6}, {X: 6, Y: 6}, {X: 5, Y: 6}, {X: 4, Y: 6}, {X: 3, Y: 6}, {X: 2, Y: 6}, {X: 1, Y: 6}, {X: 0, Y: 6}, {X: 9, Y: 5}, {X: 8, Y: 5}, {X: 7, Y: 5}, {X: 6, Y: 5}, {X: 5, Y: 5}, {X: 4, Y: 5}}
	for i, target := range targets {
		if _, _, err := m.ShootPVP(ctx, winner.ID, g.ID, target); err != nil {
			t.Fatalf("winner shot %d: %v", i, err)
		}
		if i == len(targets)-1 {
			break
		}
		if i == 0 {
			if _, _, err := m.ShootPVP(ctx, winner.ID, g.ID, targets[1]); !errors.Is(err, ErrNotYourTurn) {
				t.Fatalf("wrong-turn shot: %v", err)
			}
		}
		if _, _, err := m.ShootPVP(ctx, loser.ID, g.ID, misses[i]); err != nil {
			t.Fatalf("loser shot %d: %v", i, err)
		}
		if i == 0 {
			if _, _, err := m.ShootPVP(ctx, winner.ID, g.ID, target); err == nil || err.Error() != "duplicate shot" {
				t.Fatalf("duplicate shot: %v", err)
			}
		}
	}

	complete, err := m.PVPGame(ctx, winner.ID, g.ID)
	if err != nil || complete.Status != "complete" || complete.Winner != winner.ID || complete.RatingDelta <= 0 || complete.ShareID == "" {
		t.Fatalf("unexpected completion: %#v, %v", complete, err)
	}
	loserView, err := m.PVPGame(ctx, loser.ID, g.ID)
	if err != nil || loserView.RatingDelta >= 0 || len(loserView.Opponent.Board) != 70 {
		t.Fatalf("unexpected loser projection: %#v, %v", loserView, err)
	}
	if winnerStats, _ := m.Stats(ctx, winner.ID, "pvp"); winnerStats.Games != 1 || winnerStats.Wins != 1 {
		t.Fatalf("winner stats: %#v", winnerStats)
	}
	if loserStats, _ := m.Stats(ctx, loser.ID, "pvp"); loserStats.Games != 1 || loserStats.Losses != 1 {
		t.Fatalf("loser stats: %#v", loserStats)
	}
	if history, _ := m.PVPHistory(ctx, winner.ID, 10); len(history) != 1 || history[0].ShareID != complete.ShareID {
		t.Fatalf("winner history: %#v", history)
	}
	if share, err := m.PublicPVPShare(ctx, complete.ShareID); err != nil || share.Winner.ID != winner.ID || share.Loser.ID != loser.ID {
		t.Fatalf("share: %#v, %v", share, err)
	}
	if board, _ := m.Leaderboard(ctx, 10); len(board) != 0 {
		t.Fatalf("legacy fleet_v1 records must not enter the current PvP leaderboard: %#v", board)
	}

	rematch, err := m.Rematch(ctx, winner.ID, g.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	again, err := m.Rematch(ctx, winner.ID, g.ID, time.Now().Add(time.Hour))
	if err != nil || again.Code != rematch.Code {
		t.Fatalf("idempotent rematch: %#v, %v", again, err)
	}
	third, _ := m.UpsertGitHubUser(ctx, User{GitHubID: 13, Login: "carol"}, mockCells(time.Now()))
	if _, _, err = m.AcceptChallenge(ctx, third.ID, rematch.Code, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("third-party rematch accept: %v", err)
	}
	if _, _, err = m.AcceptChallenge(ctx, loser.ID, rematch.Code, time.Now()); err != nil {
		t.Fatalf("opponent rematch accept: %v", err)
	}
}

func TestMemoryPVPConcurrentShotsCommitOnlyOneTurn(t *testing.T) {
	m, _, _, g := newMemoryBattle(t)
	ctx := context.Background()
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for _, coord := range []game.Coord{{X: 0, Y: 0}, {X: 1, Y: 0}} {
		wg.Add(1)
		go func(c game.Coord) {
			defer wg.Done()
			_, _, err := m.ShootPVP(ctx, g.CurrentTurn, g.ID, c)
			errCh <- err
		}(coord)
	}
	wg.Wait()
	close(errCh)
	success, rejected := 0, 0
	for err := range errCh {
		if err == nil {
			success++
		} else if errors.Is(err, ErrNotYourTurn) {
			rejected++
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("success=%d rejected=%d", success, rejected)
	}
}
func TestMemoryChallengeAcceptanceIsAtomicAndSetupHidden(t *testing.T) {
	m := NewMemoryRepository()
	ctx := context.Background()
	days := mockCells(time.Now())
	a, _ := m.UpsertGitHubUser(ctx, User{GitHubID: 1, Login: "alice"}, days)
	b, _ := m.UpsertGitHubUser(ctx, User{GitHubID: 2, Login: "bob"}, days)
	c, _ := m.UpsertGitHubUser(ctx, User{GitHubID: 3, Login: "carol"}, days)
	ch, _ := m.CreateChallenge(ctx, a.ID, randomSecret(12), time.Now().Add(time.Hour))
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	for _, u := range []User{b, c} {
		go func() { defer wg.Done(); _, _, e := m.AcceptChallenge(ctx, u.ID, ch.Code, time.Now()); errs <- e }()
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("accept winners=%d", wins)
	}
	accepted, _ := m.PublicChallenge(ctx, ch.Code)
	opp := accepted.Opponent.ID
	_, g, e := m.ReadyChallenge(ctx, a.ID, ch.Code, days[0].Date, days[:70], pvpFleet(), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = m.ReadyChallenge(ctx, a.ID, ch.Code, days[0].Date, days[:70], pvpFleet(), time.Now()); e != ErrSetupLocked {
		t.Fatalf("second setup=%v", e)
	}
	_, g, e = m.ReadyChallenge(ctx, opp, ch.Code, days[0].Date, days[:70], pvpFleet(), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if len(g.Opponent.Fleet) != 0 {
		t.Fatal("opponent fleet leaked")
	}
	for _, d := range g.Opponent.Board {
		if d.Date != "" {
			t.Fatal("opponent date leaked")
		}
	}
}
func TestSelfChallengeRejected(t *testing.T) {
	m := NewMemoryRepository()
	u, _ := m.UpsertGitHubUser(context.Background(), User{GitHubID: 1, Login: "alice"}, mockCells(time.Now()))
	c, _ := m.CreateChallenge(context.Background(), u.ID, randomSecret(12), time.Now().Add(time.Hour))
	if _, _, e := m.AcceptChallenge(context.Background(), u.ID, c.Code, time.Now()); e != ErrSelfChallenge {
		t.Fatalf("got %v", e)
	}
}

func TestPublicChallengeTreatsInvalidOptionalSessionAsAnonymous(t *testing.T) {
	s, repo := testServer(t)
	u, _ := repo.UpsertGitHubUser(context.Background(), User{GitHubID: 21, Login: "alice"}, mockCells(s.now()))
	c, _ := repo.CreateChallenge(context.Background(), u.ID, "public-code", time.Now().Add(time.Hour))
	r := httptest.NewRequest(http.MethodGet, "/v1/public/challenges/"+c.Code, nil)
	r.Header.Set("Authorization", "Bearer expired-token")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPVPRateLimiterUsesFixedWindow(t *testing.T) {
	l := newRequestLimiter(2, time.Minute)
	now := time.Now()
	if !l.allow("player", now) || !l.allow("player", now) || l.allow("player", now) {
		t.Fatal("limit was not enforced")
	}
	if !l.allow("player", now.Add(time.Minute)) {
		t.Fatal("window did not reset")
	}
}

func TestPVPShareCardHasSocialDimensions(t *testing.T) {
	img, err := renderPVPShareCard(PVPShare{Winner: User{Login: "alice"}, Loser: User{Login: "bob"}, WinnerResult: PVPHistory{Shots: 31, Hits: 17, RatingDelta: 16}, Rank: "Captain"})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 {
		t.Fatalf("bounds=%v", img.Bounds())
	}
}
