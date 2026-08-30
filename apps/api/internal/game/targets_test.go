package game

import (
	"testing"
	"time"
)

func targetDays(weeks, activeEvery int) []Cell {
	start := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	out := make([]Cell, weeks*Height)
	for i := range out {
		count := 0
		if activeEvery > 0 && i%activeEvery == 0 {
			count = i%40 + 1
		}
		out[i] = Cell{Date: start.AddDate(0, 0, i).Format("2006-01-02"), Weekday: i % Height, ContributionCount: count, ContributionLevel: map[bool]int{true: 4}[count > 0]}
	}
	return out
}

func TestContributionTargetsAndSingleHitIntensity(t *testing.T) {
	board := targetDays(10, 0)
	board[0].ContributionCount, board[0].ContributionLevel = 1, 1
	board[1].ContributionCount, board[1].ContributionLevel = 40, 4
	if len(board) != 70 || TargetCount(board) != 2 {
		t.Fatal(len(board), TargetCount(board))
	}
	first, complete, err := ResolveTargetShot(board, nil, Coord{0, 0})
	if err != nil || complete || first.Result != "hit" || first.ContributionCount != 1 {
		t.Fatal(first, complete, err)
	}
	second, complete, err := ResolveTargetShot(board, []TargetShot{first}, Coord{0, 1})
	if err != nil || !complete || second.Result != "hit" || second.ContributionCount != 40 {
		t.Fatal(second, complete, err)
	}
	miss, _, _ := ResolveTargetShot(board, nil, Coord{0, 2})
	if miss.Result != "miss" || miss.ContributionCount != 0 {
		t.Fatal(miss)
	}
	if _, _, err = ResolveTargetShot(board, []TargetShot{first}, Coord{0, 0}); err == nil {
		t.Fatal("duplicate shot accepted")
	}
}

func TestWindowSelectionQualityAndFallback(t *testing.T) {
	days := targetDays(12, 3)
	w, err := SelectTargetWindow(days, 10, 45, fixed(0))
	if err != nil || len(w.Cells) != 70 || w.StartIndex%7 != 0 || w.TargetCount < 10 || w.TargetCount > 45 {
		t.Fatal(w, err)
	}
	// Sparse histories still choose the closest non-empty playable window.
	sparse := targetDays(11, 0)
	sparse[7].ContributionCount, sparse[7].ContributionLevel = 1, 1
	w, err = SelectTargetWindow(sparse, 10, 45, fixed(0))
	if err != nil || w.TargetCount != 1 {
		t.Fatal(w, err)
	}
	if _, err = SelectTargetWindow(targetDays(10, 0), 10, 45, fixed(0)); err == nil {
		t.Fatal("zero-target history should not create an instant game")
	}
}

func TestFrozenWindowDoesNotFollowLiveHistory(t *testing.T) {
	days := targetDays(10, 2)
	w, err := SelectTargetWindow(days, 10, 45, fixed(0))
	if err != nil {
		t.Fatal(err)
	}
	want := w.Cells[0].ContributionCount
	days[0].ContributionCount = 999
	if w.Cells[0].ContributionCount != want {
		t.Fatal("frozen window aliased live contribution data")
	}
}

func TestSoloRatingIsDensityAwareAndBounded(t *testing.T) {
	if SoloRatingDelta(20, 20) <= 0 || SoloRatingDelta(20, 70) < -12 || SoloRatingDelta(20, 20) > 12 {
		t.Fatal(SoloRatingDelta(20, 20), SoloRatingDelta(20, 70))
	}
}
