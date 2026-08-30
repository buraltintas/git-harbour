package game

import (
	"errors"
	"reflect"
	"testing"
)

type failingRand struct{ err error }

func (f failingRand) Intn(int) (int, error) { return 0, f.err }

type invalidRand int

func (r invalidRand) Intn(int) (int, error) { return int(r), nil }

func TestChooseStartingPlayer(t *testing.T) {
	players := [2]string{"alice", "bob"}
	for pick, want := range []string{"alice", "bob"} {
		got, err := ChooseStartingPlayer(players, fixed(pick))
		if err != nil || got != want {
			t.Fatalf("pick %d: got %q, %v; want %q", pick, got, err, want)
		}
	}

	if _, err := ChooseStartingPlayer([2]string{"alice", "alice"}, fixed(0)); err == nil {
		t.Fatal("accepted the same participant twice")
	}
	if _, err := ChooseStartingPlayer([2]string{"", "bob"}, fixed(0)); err == nil {
		t.Fatal("accepted an empty participant")
	}
	if _, err := ChooseStartingPlayer(players, nil); err == nil {
		t.Fatal("accepted a nil random source")
	}
	wantErr := errors.New("entropy unavailable")
	if _, err := ChooseStartingPlayer(players, failingRand{wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("random error = %v, want %v", err, wantErr)
	}
	if _, err := ChooseStartingPlayer(players, invalidRand(2)); err == nil {
		t.Fatal("accepted an out-of-range random result")
	}
}

func TestResolvePVPTurnMissSwitchesTurnWithoutMutatingInput(t *testing.T) {
	fleet := validFleet()
	original := validFleet()
	transition, err := ResolvePVPTurn(PVPStatusBattle, "alice", "alice", "bob", fleet, nil, Coord{9, 6})
	if err != nil {
		t.Fatal(err)
	}
	if transition.Shot.Result != "miss" || transition.NextTurnID != "bob" || transition.WinnerID != "" {
		t.Fatalf("unexpected transition: %+v", transition)
	}
	if !reflect.DeepEqual(fleet, original) {
		t.Fatal("input fleet was mutated")
	}
}

func TestResolvePVPTurnHitAndSunkReuseSharedRules(t *testing.T) {
	fleet := validFleet()
	hit, err := ResolvePVPTurn(PVPStatusBattle, "alice", "alice", "bob", fleet, nil, Coord{0, 4})
	if err != nil {
		t.Fatal(err)
	}
	if hit.Shot.Result != "hit" || hit.Shot.Ship != "Destroyer" || hit.NextTurnID != "bob" {
		t.Fatalf("unexpected hit transition: %+v", hit)
	}

	// A later legal turn by the same shooter sinks the two-cell ship. The hit
	// history prevents repeats while the persisted fleet carries prior damage.
	sunk, err := ResolvePVPTurn(PVPStatusBattle, "alice", "alice", "bob", hit.TargetFleet, []Shot{hit.Shot}, Coord{1, 4})
	if err != nil {
		t.Fatal(err)
	}
	if sunk.Shot.Result != "sunk" || sunk.Shot.Ship != "Destroyer" || sunk.NextTurnID != "bob" || sunk.WinnerID != "" {
		t.Fatalf("unexpected sunk transition: %+v", sunk)
	}
}

func TestResolvePVPTurnVictoryDoesNotAssignAnotherTurn(t *testing.T) {
	fleet := validFleet()
	for i := range fleet {
		fleet[i].Hits = append([]Coord(nil), fleet[i].Cells...)
	}
	// Leave the final Destroyer cell unhit.
	fleet[4].Hits = fleet[4].Hits[:1]
	previous := []Shot{{Coord: fleet[4].Hits[0], Result: "hit", Ship: "Destroyer"}}

	transition, err := ResolvePVPTurn(PVPStatusBattle, "alice", "alice", "bob", fleet, previous, fleet[4].Cells[1])
	if err != nil {
		t.Fatal(err)
	}
	if transition.Shot.Result != "sunk" || transition.WinnerID != "alice" || transition.NextTurnID != "" {
		t.Fatalf("unexpected terminal transition: %+v", transition)
	}
}

func TestResolvePVPTurnRejectsInvalidStateAndIntent(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		current  string
		shooter  string
		opponent string
		shots    []Shot
		target   Coord
	}{
		{name: "complete game", status: "complete", current: "alice", shooter: "alice", opponent: "bob", target: Coord{}},
		{name: "wrong turn", status: PVPStatusBattle, current: "bob", shooter: "alice", opponent: "bob", target: Coord{}},
		{name: "same participant", status: PVPStatusBattle, current: "alice", shooter: "alice", opponent: "alice", target: Coord{}},
		{name: "empty participant", status: PVPStatusBattle, current: "", shooter: "", opponent: "bob", target: Coord{}},
		{name: "invalid fleet", status: PVPStatusBattle, current: "alice", shooter: "alice", opponent: "bob", target: Coord{}},
		{name: "out of bounds", status: PVPStatusBattle, current: "alice", shooter: "alice", opponent: "bob", target: Coord{X: Width}},
		{name: "duplicate shot", status: PVPStatusBattle, current: "alice", shooter: "alice", opponent: "bob", shots: []Shot{{Coord: Coord{9, 6}, Result: "miss"}}, target: Coord{9, 6}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fleet := validFleet()
			if tc.name == "invalid fleet" {
				fleet = nil
			}
			if _, err := ResolvePVPTurn(tc.status, tc.current, tc.shooter, tc.opponent, fleet, tc.shots, tc.target); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestUpdateStatsAgainstUsesBothPreMatchRatings(t *testing.T) {
	winnerBefore := Stats{Games: 4, Wins: 3, Losses: 1, Rating: 1450, Shots: 80, Hits: 30, CurrentStreak: 2, LongestStreak: 2, WinShots: 60}
	loserBefore := Stats{Games: 7, Wins: 4, Losses: 3, Rating: 1200, Shots: 140, Hits: 50, CurrentStreak: 3, LongestStreak: 4, WinShots: 90}

	winner := UpdateStatsAgainst(winnerBefore, loserBefore.Rating, true, 27, 17)
	loser := UpdateStatsAgainst(loserBefore, winnerBefore.Rating, false, 31, 12)
	if winner.Rating != Elo(1450, 1200, true) || loser.Rating != Elo(1200, 1450, false) {
		t.Fatalf("ratings did not use pre-match values: winner=%d loser=%d", winner.Rating, loser.Rating)
	}
	if winner.Games != 5 || winner.Wins != 4 || winner.Losses != 1 || winner.Shots != 107 || winner.Hits != 47 || winner.CurrentStreak != 3 || winner.LongestStreak != 3 || winner.WinShots != 87 {
		t.Fatalf("winner stats incorrect: %+v", winner)
	}
	if loser.Games != 8 || loser.Wins != 4 || loser.Losses != 4 || loser.Shots != 171 || loser.Hits != 62 || loser.CurrentStreak != 0 || loser.LongestStreak != 4 || loser.WinShots != 90 {
		t.Fatalf("loser stats incorrect: %+v", loser)
	}
	if winnerBefore.Rating != 1450 || loserBefore.Rating != 1200 {
		t.Fatal("input stats were mutated")
	}
}

func TestUpdateStatsKeepsSoloFixedOpponentBehavior(t *testing.T) {
	before := Stats{Rating: 1200}
	if got, want := UpdateStats(before, true, 20, 8), UpdateStatsAgainst(before, 1200, true, 20, 8); got != want {
		t.Fatalf("Solo behavior changed: got %+v want %+v", got, want)
	}
	if got := UpdateStats(Stats{Rating: 1450}, false, 20, 8); got.Rating != Elo(1450, 1200, false) {
		t.Fatalf("Solo no longer uses a 1200 opponent: %+v", got)
	}
}
