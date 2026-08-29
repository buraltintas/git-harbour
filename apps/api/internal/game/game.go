package game

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

const Width, Height = 10, 7

type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type Cell struct {
	Date              string `json:"date"`
	Weekday           int    `json:"weekday"`
	ContributionCount int    `json:"contributionCount"`
	ContributionLevel int    `json:"contributionLevel"`
}
type Ship struct {
	Kind  string  `json:"kind"`
	Cells []Coord `json:"cells"`
	Hits  []Coord `json:"hits,omitempty"`
}
type Shot struct {
	Coord
	Result string `json:"result"`
	Ship   string `json:"ship,omitempty"`
}
type Stats struct{ Games, Wins, Losses, Rating, Shots, Hits, CurrentStreak, LongestStreak, WinShots int }

var Fleet = map[string]int{"Carrier": 5, "Battleship": 4, "Cruiser": 3, "Submarine": 3, "Destroyer": 2}

func key(c Coord) string    { return fmt.Sprintf("%d:%d", c.X, c.Y) }
func InBounds(c Coord) bool { return c.X >= 0 && c.X < Width && c.Y >= 0 && c.Y < Height }

func ValidateFleet(f []Ship) error {
	if len(f) != len(Fleet) {
		return errors.New("fleet must contain five ships")
	}
	seen, kinds := map[string]bool{}, map[string]bool{}
	for _, s := range f {
		n, ok := Fleet[s.Kind]
		if !ok || kinds[s.Kind] {
			return errors.New("invalid or duplicate ship kind")
		}
		kinds[s.Kind] = true
		if len(s.Cells) != n {
			return fmt.Errorf("%s must have %d cells", s.Kind, n)
		}
		xs, ys := map[int]bool{}, map[int]bool{}
		for _, c := range s.Cells {
			if !InBounds(c) {
				return errors.New("ship out of bounds")
			}
			if seen[key(c)] {
				return errors.New("ships overlap")
			}
			seen[key(c)] = true
			xs[c.X] = true
			ys[c.Y] = true
		}
		if len(xs) != 1 && len(ys) != 1 {
			return errors.New("ships must be straight")
		}
		cc := append([]Coord(nil), s.Cells...)
		sort.Slice(cc, func(i, j int) bool {
			if cc[i].X == cc[j].X {
				return cc[i].Y < cc[j].Y
			}
			return cc[i].X < cc[j].X
		})
		for i := 1; i < len(cc); i++ {
			if abs(cc[i].X-cc[i-1].X)+abs(cc[i].Y-cc[i-1].Y) != 1 {
				return errors.New("ship cells must be contiguous")
			}
		}
	}
	return nil
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func ResolveShot(fleet []Ship, previous []Shot, c Coord) (Shot, []Ship, error) {
	if !InBounds(c) {
		return Shot{}, fleet, errors.New("shot out of bounds")
	}
	for _, s := range previous {
		if s.Coord == c {
			return Shot{}, fleet, errors.New("duplicate shot")
		}
	}
	result := Shot{Coord: c, Result: "miss"}
	next := append([]Ship(nil), fleet...)
	for i, ship := range next {
		next[i].Cells = append([]Coord(nil), ship.Cells...)
		next[i].Hits = append([]Coord(nil), ship.Hits...)
		for _, p := range ship.Cells {
			if p == c {
				next[i].Hits = append(next[i].Hits, c)
				result.Result = "hit"
				result.Ship = ship.Kind
				if len(next[i].Hits) == len(ship.Cells) {
					result.Result = "sunk"
				}
				return result, next, nil
			}
		}
	}
	return result, next, nil
}
func AllSunk(f []Ship) bool {
	for _, s := range f {
		if len(s.Hits) < len(s.Cells) {
			return false
		}
	}
	return true
}

type Rander interface{ Intn(int) (int, error) }
type SecureRand struct{}

func (SecureRand) Intn(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("invalid bound")
	}
	var b [8]byte
	if _, e := rand.Read(b[:]); e != nil {
		return 0, e
	}
	return int(binary.BigEndian.Uint64(b[:]) % uint64(n)), nil
}

func OpponentStart(totalWeeks, selected int, r Rander) (int, error) {
	if totalWeeks < 20 {
		return 0, errors.New("at least 20 weeks required")
	}
	max := totalWeeks - Width + 1
	n, e := r.Intn(max - 1)
	if e != nil {
		return 0, e
	}
	if n >= selected {
		n++
	}
	return n, nil
}

func PlaceFleet(r Rander) ([]Ship, error) {
	for attempts := 0; attempts < 1000; attempts++ {
		ships := []Ship{}
		occupied := map[string]bool{}
		ok := true
		for _, kind := range []string{"Carrier", "Battleship", "Cruiser", "Submarine", "Destroyer"} {
			size := Fleet[kind]
			placed := false
			for tries := 0; tries < 200; tries++ {
				vertical, e := r.Intn(2)
				if e != nil {
					return nil, e
				}
				maxX, maxY := Width, Height
				if vertical == 0 {
					maxX = Width - size + 1
				} else {
					maxY = Height - size + 1
				}
				x, _ := r.Intn(maxX)
				y, _ := r.Intn(maxY)
				cells := []Coord{}
				valid := true
				for i := 0; i < size; i++ {
					c := Coord{x, y}
					if vertical == 0 {
						c.X += i
					} else {
						c.Y += i
					}
					if occupied[key(c)] {
						valid = false
					}
					cells = append(cells, c)
				}
				if valid {
					for _, c := range cells {
						occupied[key(c)] = true
					}
					ships = append(ships, Ship{Kind: kind, Cells: cells})
					placed = true
					break
				}
			}
			if !placed {
				ok = false
				break
			}
		}
		if ok {
			return ships, nil
		}
	}
	return nil, errors.New("could not place fleet")
}

func NextAITarget(shots []Shot, r Rander) (Coord, error) {
	used := map[string]bool{}
	sunk := map[string]bool{}
	for _, s := range shots {
		if s.Result == "sunk" {
			sunk[s.Ship] = true
		}
	}
	hits := []Coord{}
	for _, s := range shots {
		used[key(s.Coord)] = true
		if s.Result == "hit" && !sunk[s.Ship] {
			hits = append(hits, s.Coord)
		}
	}
	candidates := []Coord{}
	if len(hits) >= 2 {
		last := hits[len(hits)-1]
		for i := len(hits) - 2; i >= 0; i-- {
			h := hits[i]
			if h.X == last.X {
				candidates = append(candidates, Coord{last.X, last.Y + (last.Y - h.Y)}, Coord{h.X, h.Y - (last.Y - h.Y)})
				break
			}
			if h.Y == last.Y {
				candidates = append(candidates, Coord{last.X + (last.X - h.X), last.Y}, Coord{h.X - (last.X - h.X), h.Y})
				break
			}
		}
	}
	if len(candidates) == 0 && len(hits) > 0 {
		h := hits[len(hits)-1]
		candidates = []Coord{{h.X + 1, h.Y}, {h.X - 1, h.Y}, {h.X, h.Y + 1}, {h.X, h.Y - 1}}
	}
	valid := []Coord{}
	for _, c := range candidates {
		if InBounds(c) && !used[key(c)] {
			valid = append(valid, c)
		}
	}
	if len(valid) > 0 {
		n, e := r.Intn(len(valid))
		return valid[n], e
	}
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			c := Coord{x, y}
			if !used[key(c)] {
				valid = append(valid, c)
			}
		}
	}
	if len(valid) == 0 {
		return Coord{}, errors.New("no targets")
	}
	n, e := r.Intn(len(valid))
	return valid[n], e
}

func Elo(rating, opponent int, won bool) int {
	score := 0.0
	if won {
		score = 1
	}
	expected := 1 / (1 + math.Pow(10, float64(opponent-rating)/400))
	return int(float64(rating) + 32*(score-expected) + 0.5)
}
func UpdateStats(s Stats, won bool, shots, hits int) Stats {
	s.Games++
	s.Shots += shots
	s.Hits += hits
	if won {
		s.Wins++
		s.CurrentStreak++
		s.WinShots += shots
		if s.CurrentStreak > s.LongestStreak {
			s.LongestStreak = s.CurrentStreak
		}
	} else {
		s.Losses++
		s.CurrentStreak = 0
	}
	s.Rating = Elo(s.Rating, 1200, won)
	return s
}
func DateRange(start time.Time) (string, string) {
	return start.Format("2006-01-02"), start.AddDate(0, 0, 69).Format("2006-01-02")
}
