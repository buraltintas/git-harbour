package game

import "testing"

type fixed int

func (f fixed) Intn(n int) (int, error) { return int(f) % n, nil }
func validFleet() []Ship {
	return []Ship{{"Carrier", []Coord{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}}, nil}, {"Battleship", []Coord{{0, 1}, {1, 1}, {2, 1}, {3, 1}}, nil}, {"Cruiser", []Coord{{0, 2}, {1, 2}, {2, 2}}, nil}, {"Submarine", []Coord{{0, 3}, {1, 3}, {2, 3}}, nil}, {"Destroyer", []Coord{{0, 4}, {1, 4}}, nil}}
}
func TestFleetValidation(t *testing.T) {
	if e := ValidateFleet(validFleet()); e != nil {
		t.Fatal(e)
	}
	f := validFleet()
	f[4].Cells[0] = f[0].Cells[0]
	if ValidateFleet(f) == nil {
		t.Fatal("expected overlap")
	}
	f = validFleet()
	f[0].Cells[4] = Coord{10, 0}
	if ValidateFleet(f) == nil {
		t.Fatal("expected bounds")
	}
}
func TestShotsAndVictory(t *testing.T) {
	f := validFleet()
	s, n, e := ResolveShot(f, nil, Coord{0, 0})
	if e != nil || s.Result != "hit" {
		t.Fatal(s, e)
	}
	if _, _, e = ResolveShot(n, []Shot{s}, Coord{0, 0}); e == nil {
		t.Fatal("expected duplicate")
	}
	m, _, _ := ResolveShot(n, []Shot{s}, Coord{9, 6})
	if m.Result != "miss" {
		t.Fatal(m)
	}
	for i := 1; i < 5; i++ {
		s, n, _ = ResolveShot(n, nil, Coord{i, 0})
	}
	if s.Result != "sunk" {
		t.Fatal(s)
	}
	for i := 1; i < len(n); i++ {
		n[i].Hits = append([]Coord(nil), n[i].Cells...)
	}
	if !AllSunk(n) {
		t.Fatal("expected victory")
	}
}
func TestOpponentAndAI(t *testing.T) {
	n, e := OpponentStart(52, 0, fixed(0))
	if e != nil || n == 0 || n > 42 {
		t.Fatal(n, e)
	}
	shots := []Shot{{Coord: Coord{1, 1}, Result: "miss"}, {Coord: Coord{2, 2}, Result: "hit"}}
	c, e := NextAITarget(shots, fixed(0))
	if e != nil || c == (Coord{1, 1}) || c == (Coord{2, 2}) {
		t.Fatal(c, e)
	}
}
func TestEloAndStatsOnceInput(t *testing.T) {
	if Elo(1200, 1200, true) != 1216 || Elo(1200, 1200, false) != 1184 {
		t.Fatal("elo")
	}
	s := UpdateStats(Stats{Rating: 1200}, true, 20, 8)
	if s.Games != 1 || s.Wins != 1 || s.Rating != 1216 {
		t.Fatal(s)
	}
}
