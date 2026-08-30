package game

import (
	"errors"
	"math"
	"time"
)

const (
	BoardCells      = Width * Height
	IdealMinTargets = 10
	IdealMaxTargets = 45
)

type TargetWindow struct {
	StartIndex  int
	Cells       []Cell
	TargetCount int
}

type TargetShot struct {
	Coord
	Result            string `json:"result"`
	ContributionCount int    `json:"contributionCount,omitempty"`
	ContributionLevel int    `json:"contributionLevel,omitempty"`
}

type BattleEvent struct {
	Actor string `json:"actor"`
	TargetShot
}

func TargetCount(cells []Cell) int {
	n := 0
	for _, cell := range cells {
		if cell.ContributionCount > 0 {
			n++
		}
	}
	return n
}

func contiguous(cells []Cell) bool {
	if len(cells) != BoardCells || cells[0].Weekday != 0 {
		return false
	}
	first, err := time.Parse("2006-01-02", cells[0].Date)
	if err != nil || int(first.Weekday()) != cells[0].Weekday {
		return false
	}
	for i := 1; i < len(cells); i++ {
		before, e1 := time.Parse("2006-01-02", cells[i-1].Date)
		after, e2 := time.Parse("2006-01-02", cells[i].Date)
		if e1 != nil || e2 != nil || !before.AddDate(0, 0, 1).Equal(after) || cells[i].Weekday != int(after.Weekday()) {
			return false
		}
	}
	return true
}

// SelectTargetWindow chooses a week-aligned, contiguous ten-week window. It
// prefers boards in the documented quality range and otherwise chooses the
// closest playable density. Random choice among equal candidates prevents a
// user's latest or densest period from becoming predictable.
func SelectTargetWindow(days []Cell, minTargets, maxTargets int, r Rander) (TargetWindow, error) {
	if minTargets < 1 || maxTargets < minTargets || r == nil {
		return TargetWindow{}, errors.New("invalid board quality configuration")
	}
	candidates := []TargetWindow{}
	for start := 0; start+BoardCells <= len(days); start += Height {
		cells := days[start : start+BoardCells]
		if !contiguous(cells) {
			continue
		}
		count := TargetCount(cells)
		if count > 0 {
			candidates = append(candidates, TargetWindow{StartIndex: start, Cells: append([]Cell(nil), cells...), TargetCount: count})
		}
	}
	if len(candidates) == 0 {
		return TargetWindow{}, errors.New("contribution history has no playable ten-week window")
	}
	ideal := []TargetWindow{}
	for _, candidate := range candidates {
		if candidate.TargetCount >= minTargets && candidate.TargetCount <= maxTargets {
			ideal = append(ideal, candidate)
		}
	}
	if len(ideal) > 0 {
		candidates = ideal
	} else {
		best := math.MaxInt
		closest := []TargetWindow{}
		for _, candidate := range candidates {
			distance := 0
			if candidate.TargetCount < minTargets {
				distance = minTargets - candidate.TargetCount
			} else if candidate.TargetCount > maxTargets {
				distance = candidate.TargetCount - maxTargets
			}
			if distance < best {
				best, closest = distance, []TargetWindow{candidate}
			} else if distance == best {
				closest = append(closest, candidate)
			}
		}
		candidates = closest
	}
	index, err := r.Intn(len(candidates))
	if err != nil || index < 0 || index >= len(candidates) {
		return TargetWindow{}, errors.New("could not choose contribution window")
	}
	return candidates[index], nil
}

// TargetWindowAt freezes the week-aligned ten-week period beginning at start.
// The client selects only the date; the authoritative cells always come from
// the server's imported contribution history.
func TargetWindowAt(days []Cell, start string) (TargetWindow, error) {
	all := []TargetWindow{}
	selected := TargetWindow{StartIndex: -1}
	for i := 0; i+BoardCells <= len(days); i += Height {
		cells := days[i : i+BoardCells]
		if !contiguous(cells) {
			continue
		}
		count := TargetCount(cells)
		if count > 0 {
			candidate := TargetWindow{StartIndex: i, Cells: append([]Cell(nil), cells...), TargetCount: count}
			all = append(all, candidate)
			if cells[0].Date == start {
				selected = candidate
			}
		}
	}
	if selected.StartIndex < 0 {
		return TargetWindow{}, errors.New("selected contribution window is invalid")
	}
	best := math.MaxInt
	for _, candidate := range all {
		if distance := qualityDistance(candidate.TargetCount); distance < best {
			best = distance
		}
	}
	if qualityDistance(selected.TargetCount) != best {
		return TargetWindow{}, errors.New("selected contribution window is outside the playable density range")
	}
	return selected, nil
}

func qualityDistance(targets int) int {
	if targets < IdealMinTargets {
		return IdealMinTargets - targets
	}
	if targets > IdealMaxTargets {
		return targets - IdealMaxTargets
	}
	return 0
}

// SelectFairOpponentWindow chooses a different playable period without
// inspecting it during combat. It first prefers non-overlapping windows within
// five target days, then minimizes target-count difference and securely picks
// among ties. If no fair non-overlapping window exists, all different windows
// participate in the closest-density fallback.
func SelectFairOpponentWindow(days []Cell, player TargetWindow, r Rander) (TargetWindow, error) {
	if r == nil {
		return TargetWindow{}, errors.New("random source is required")
	}
	candidates := []TargetWindow{}
	preferred := []TargetWindow{}
	for start := 0; start+BoardCells <= len(days); start += Height {
		if start == player.StartIndex {
			continue
		}
		cells := days[start : start+BoardCells]
		if !contiguous(cells) {
			continue
		}
		count := TargetCount(cells)
		if count == 0 {
			continue
		}
		candidate := TargetWindow{StartIndex: start, Cells: append([]Cell(nil), cells...), TargetCount: count}
		candidates = append(candidates, candidate)
		distance := absTarget(count - player.TargetCount)
		if distance <= 5 && absTarget(start-player.StartIndex) >= BoardCells {
			preferred = append(preferred, candidate)
		}
	}
	if len(candidates) == 0 {
		return TargetWindow{}, errors.New("contribution history has no different opponent window")
	}
	pool := candidates
	if len(preferred) > 0 {
		pool = preferred
	}
	bestDistance := math.MaxInt
	closest := []TargetWindow{}
	for _, candidate := range pool {
		distance := absTarget(candidate.TargetCount - player.TargetCount)
		if distance < bestDistance {
			bestDistance, closest = distance, []TargetWindow{candidate}
		} else if distance == bestDistance {
			closest = append(closest, candidate)
		}
	}
	index, err := r.Intn(len(closest))
	if err != nil || index < 0 || index >= len(closest) {
		return TargetWindow{}, errors.New("could not choose opponent contribution window")
	}
	return closest[index], nil
}

func absTarget(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// NextTarget selects only from coordinates absent from prior shots. It accepts
// no board data, which makes hidden-target inspection impossible by contract.
func NextTarget(previous []TargetShot, r Rander) (Coord, error) {
	if r == nil {
		return Coord{}, errors.New("random source is required")
	}
	used := map[Coord]bool{}
	for _, shot := range previous {
		used[shot.Coord] = true
	}
	available := make([]Coord, 0, BoardCells-len(used))
	for x := 0; x < Width; x++ {
		for y := 0; y < Height; y++ {
			c := Coord{X: x, Y: y}
			if !used[c] {
				available = append(available, c)
			}
		}
	}
	if len(available) == 0 {
		return Coord{}, errors.New("no targets remain")
	}
	index, err := r.Intn(len(available))
	if err != nil || index < 0 || index >= len(available) {
		return Coord{}, errors.New("could not choose target")
	}
	return available[index], nil
}

func ResolveTargetShot(board []Cell, previous []TargetShot, c Coord) (TargetShot, bool, error) {
	if len(board) != BoardCells {
		return TargetShot{}, false, errors.New("board must contain 70 cells")
	}
	if !InBounds(c) {
		return TargetShot{}, false, errors.New("shot out of bounds")
	}
	seen := map[Coord]bool{}
	for _, prior := range previous {
		if !InBounds(prior.Coord) || seen[prior.Coord] {
			return TargetShot{}, false, errors.New("invalid previous shots")
		}
		seen[prior.Coord] = true
		priorCell := board[prior.X*Height+prior.Y]
		expected := "miss"
		if priorCell.ContributionCount > 0 {
			expected = "hit"
		}
		if prior.Result != expected || (expected == "hit" && (prior.ContributionCount != priorCell.ContributionCount || prior.ContributionLevel != priorCell.ContributionLevel)) {
			return TargetShot{}, false, errors.New("previous shot results do not match frozen board")
		}
		if prior.Coord == c {
			return TargetShot{}, false, errors.New("duplicate shot")
		}
	}
	cell := board[c.X*Height+c.Y]
	shot := TargetShot{Coord: c, Result: "miss"}
	if cell.ContributionCount > 0 {
		shot.Result = "hit"
		shot.ContributionCount = cell.ContributionCount
		shot.ContributionLevel = cell.ContributionLevel
	}
	hits := 0
	for _, prior := range previous {
		if prior.Result == "hit" {
			hits++
		}
	}
	if shot.Result == "hit" {
		hits++
	}
	return shot, hits == TargetCount(board), nil
}
