package game

import (
	"math"
	"testing"
	"time"
)

type sequenceRand struct {
	values []int
	at     int
}

func (r *sequenceRand) Intn(n int) (int, error) {
	v := r.values[r.at%len(r.values)]
	r.at++
	return v % n, nil
}

func fleetBoard(counts map[int]int) []Cell {
	start := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	cells := make([]Cell, BoardCells)
	for i := range cells {
		cells[i] = Cell{Date: start.AddDate(0, 0, i).Format("2006-01-02"), Weekday: i % 7, ContributionCount: counts[i]}
	}
	return cells
}

func TestDayPowerIsMonotonicAndDiminishing(t *testing.T) {
	values := []int{1, 10, 100, 600, 6000, 1_000_000_000}
	last := 0.0
	for _, count := range values {
		p := DayPower(count)
		if p <= last {
			t.Fatalf("power not monotonic at %d", count)
		}
		last = p
	}
	if DayPower(600) >= 600*DayPower(1) {
		t.Fatal("power must use diminishing returns")
	}
	if DayPower(0) != 0 || DayPower(-1) != 0 {
		t.Fatal("inactive days must not gain contribution power")
	}
}

func TestFleetCapacityCurve(t *testing.T) {
	wants := map[int]int{0: 3, 1: 4, 10: 7, 30: 10, 50: 12, 70: 14}
	last := 0
	for active := 0; active <= 70; active++ {
		got := FleetCapacity(active)
		if got < last {
			t.Fatal("capacity decreased")
		}
		if got > MaximumFleetCapacity {
			t.Fatal("capacity exceeds limit")
		}
		last = got
	}
	for active, want := range wants {
		if got := FleetCapacity(active); got != want {
			t.Fatalf("active %d: got %d want %d", active, got, want)
		}
	}
}

func TestFleetProfilesAndVeryLargeCounts(t *testing.T) {
	cases := []map[int]int{
		{}, {0: 1}, {0: 600},
		func() map[int]int {
			m := map[int]int{}
			for i := 0; i < 30; i++ {
				m[i] = 3
			}
			m[0] = 13
			return m
		}(),
		func() map[int]int {
			m := map[int]int{}
			for i := 0; i < 10; i++ {
				m[i] = 50
			}
			return m
		}(),
		func() map[int]int {
			m := map[int]int{}
			for i := 0; i < 40; i++ {
				m[i] = 1
			}
			for i := 0; i < 10; i++ {
				m[i]++
			}
			return m
		}(),
		func() map[int]int {
			m := map[int]int{}
			for i := 0; i < 70; i++ {
				m[i] = 1
			}
			return m
		}(),
		func() map[int]int {
			m := map[int]int{}
			for i := 0; i < 55; i++ {
				m[i] = 20
			}
			m[0] = 1_000_000_000
			return m
		}(),
	}
	for i, counts := range cases {
		w, err := SummarizeFleetWindow(fleetBoard(counts), 0)
		if err != nil || w.FleetCapacity < 3 || math.IsInf(w.ContributionPower, 0) {
			t.Fatalf("case %d invalid: %+v %v", i, w, err)
		}
	}
}

func TestFleetWindowAllowsZeroHistoryAndUsesOnlyAlternative(t *testing.T) {
	first := fleetBoard(nil)
	start, _ := time.Parse("2006-01-02", first[len(first)-1].Date)
	second := make([]Cell, BoardCells)
	for i := range second {
		d := start.AddDate(0, 0, i+1)
		second[i] = Cell{Date: d.Format("2006-01-02"), Weekday: int(d.Weekday())}
	}
	days := append(append([]Cell(nil), first...), second...)
	player, err := FleetWindowAt(days, first[0].Date)
	if err != nil || player.ActiveDays != 0 || player.FleetCapacity != 3 {
		t.Fatal(player, err)
	}
	opponent, err := SelectFleetOpponentWindow(days, player, &sequenceRand{values: []int{0}})
	if err != nil || opponent.StartIndex != 70 {
		t.Fatal("single alternative not used", opponent, err)
	}
	if _, err = SelectFleetOpponentWindow(first, player, &sequenceRand{values: []int{0}}); err == nil {
		t.Fatal("missing alternative must remain explicit")
	}
}

func TestFleetWindowsFindWeekAlignmentAfterMidweekHistoryStart(t *testing.T) {
	start := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC) // Thursday
	days := make([]Cell, 143)
	for i := range days {
		d := start.AddDate(0, 0, i)
		days[i] = Cell{Date: d.Format("2006-01-02"), Weekday: int(d.Weekday())}
	}

	player, err := FleetWindowAt(days, "2025-01-05")
	if err != nil || player.StartIndex != 3 || player.Cells[0].Weekday != 0 {
		t.Fatalf("midweek-aligned player window not found: %+v %v", player, err)
	}
	opponent, err := SelectFleetOpponentWindow(days, player, &sequenceRand{values: []int{0}})
	if err != nil || opponent.StartIndex != 73 {
		t.Fatalf("midweek-aligned opponent window not found: %+v %v", opponent, err)
	}
}

func TestReserveGenerationAndContributionCoordinateIntegrity(t *testing.T) {
	board := fleetBoard(map[int]int{0: 600})
	units, err := ValidateDeployment(board, []DeploymentChoice{
		{Coord: Coord{0, 0}, Kind: "contribution"},
		{Coord: Coord{0, 1}, Kind: "reserve"},
		{Coord: Coord{0, 2}, Kind: "reserve"},
		{Coord: Coord{0, 3}, Kind: "reserve"},
	})
	if err != nil || len(units) != 4 || units[0].Power != DayPower(600) {
		t.Fatalf("unexpected deployment: %+v %v", units, err)
	}
	if _, err = ValidateDeployment(board, []DeploymentChoice{{Coord: Coord{0, 1}, Kind: "contribution"}, {Coord: Coord{0, 2}, Kind: "reserve"}, {Coord: Coord{0, 3}, Kind: "reserve"}, {Coord: Coord{0, 4}, Kind: "reserve"}}); err == nil {
		t.Fatal("contribution unit moved away from real coordinate")
	}
	zero, err := RandomDeployment(fleetBoard(nil), &sequenceRand{values: []int{0}})
	if err != nil || len(zero) != 3 {
		t.Fatalf("zero-history reserve fleet failed: %+v %v", zero, err)
	}
	for _, unit := range zero {
		if unit.Kind != "reserve" || unit.Power != ReservePower {
			t.Fatal("fake contribution generated")
		}
	}
}

func TestCombatProbabilityAndEliminationExposure(t *testing.T) {
	if p := WinProbability(4, 4); p != 0.5 {
		t.Fatalf("equal power=%f", p)
	}
	if WinProbability(6, 4) <= WinProbability(5, 4) {
		t.Fatal("stronger attacker must have better odds")
	}
	if WinProbability(1, 100) != MinimumWinChance || WinProbability(100, 1) != MaximumWinChance {
		t.Fatal("probability bounds failed")
	}
	a := []FleetUnit{{Coord: Coord{0, 0}, Kind: "reserve", Power: 1, Alive: true}}
	d := []FleetUnit{{Coord: Coord{1, 1}, Kind: "contribution", Power: 2, Alive: true}}
	event, na, nd, err := ResolveFleetAction(a, d, nil, "player", Coord{0, 0}, Coord{1, 1}, &sequenceRand{values: []int{0}})
	if err != nil || event.Result != "clash" || !event.AttackerWon || !na[0].Exposed || !nd[0].Exposed || nd[0].Alive {
		t.Fatalf("attacker win wrong: %+v %+v %+v %v", event, na, nd, err)
	}
	event, na, nd, err = ResolveFleetAction(a, d, nil, "player", Coord{0, 0}, Coord{1, 1}, &sequenceRand{values: []int{999999}})
	if err != nil || event.AttackerWon || na[0].Alive || !nd[0].Alive {
		t.Fatal("defender win must eliminate attacker")
	}
}

func TestMissDuplicateImmutableAndComputerKnowledgeBoundary(t *testing.T) {
	a := []FleetUnit{{Coord: Coord{0, 0}, Kind: "reserve", Power: 1, Alive: true}}
	d := []FleetUnit{{Coord: Coord{1, 1}, Kind: "reserve", Power: 1, Alive: true}}
	event, na, nd, err := ResolveFleetAction(a, d, nil, "player", Coord{0, 0}, Coord{2, 2}, &sequenceRand{values: []int{0}})
	if err != nil || event.Result != "miss" || !na[0].Alive || !nd[0].Alive || !na[0].Exposed {
		t.Fatal("miss semantics failed")
	}
	if a[0].Exposed {
		t.Fatal("input deployment mutated")
	}
	if _, _, _, err = ResolveFleetAction(a, d, []FleetAction{event}, "player", Coord{0, 0}, Coord{2, 2}, &sequenceRand{values: []int{0}}); err == nil {
		t.Fatal("duplicate target accepted")
	}
	clash, _, survivor, err := ResolveFleetAction(a, d, nil, "player", Coord{0, 0}, Coord{1, 1}, &sequenceRand{values: []int{999999}})
	if err != nil || !survivor[0].Alive || !survivor[0].Exposed {
		t.Fatal("defender should survive exposed", err)
	}
	if _, _, _, err = ResolveFleetAction(a, survivor, []FleetAction{clash}, "player", Coord{0, 0}, Coord{1, 1}, &sequenceRand{values: []int{0}}); err != nil {
		t.Fatal("exposed surviving defender must be retargetable", err)
	}
	attacker, target, err := NextComputerAction(a, []FleetAction{{Actor: "player", Attacker: Coord{4, 4}, Target: Coord{6, 6}, Result: "miss"}}, &sequenceRand{values: []int{0, 0}})
	if err != nil || attacker != (Coord{0, 0}) || target != (Coord{4, 4}) {
		t.Fatalf("AI did not use exposed public knowledge: %v %v %v", attacker, target, err)
	}
}
