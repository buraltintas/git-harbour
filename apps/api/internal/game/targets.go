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
	if len(cells) != BoardCells {
		return false
	}
	for i := 1; i < len(cells); i++ {
		before, e1 := time.Parse("2006-01-02", cells[i-1].Date)
		after, e2 := time.Parse("2006-01-02", cells[i].Date)
		if e1 != nil || e2 != nil || !before.AddDate(0, 0, 1).Equal(after) {
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

func ResolveTargetShot(board []Cell, previous []TargetShot, c Coord) (TargetShot, bool, error) {
	if len(board) != BoardCells {
		return TargetShot{}, false, errors.New("board must contain 70 cells")
	}
	if !InBounds(c) {
		return TargetShot{}, false, errors.New("shot out of bounds")
	}
	for _, prior := range previous {
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

// SoloRatingDelta compares the player's shots with the expected position of
// the last target in a random ordering. It is density-aware and deliberately
// bounded, so sparse histories are not punished merely for being sparse.
func SoloRatingDelta(targets, shots int) int {
	if targets <= 0 || shots < targets {
		return 0
	}
	expected := int(math.Ceil(float64(BoardCells*targets) / float64(targets+1)))
	delta := int(math.Round(float64(expected-shots) / 3))
	if delta < -12 {
		return -12
	}
	if delta > 12 {
		return 12
	}
	return delta
}
