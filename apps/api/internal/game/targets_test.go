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

func TestPlayerWindowAndFairDifferentOpponent(t *testing.T) {
	days := targetDays(24, 4)
	player, err := TargetWindowAt(days, days[14].Date)
	if err != nil || player.StartIndex != 14 || len(player.Cells) != BoardCells {
		t.Fatal(player, err)
	}
	days[14].ContributionCount = 999
	if player.Cells[0].ContributionCount == 999 {
		t.Fatal("player snapshot was not frozen")
	}
	enemy, err := SelectFairOpponentWindow(days, player, fixed(0))
	if err != nil || enemy.StartIndex == player.StartIndex || absTarget(enemy.TargetCount-player.TargetCount) > 5 || absTarget(enemy.StartIndex-player.StartIndex) < BoardCells {
		t.Fatal(enemy, err)
	}
	if _, err = TargetWindowAt(days, "1900-01-01"); err == nil {
		t.Fatal("invalid player period accepted")
	}
}

func TestPlayerSelectionUsesBestAvailableQuality(t *testing.T) {
	days := targetDays(30, 0)
	days[0].ContributionCount, days[0].ContributionLevel = 1, 1
	for i := 70; i < 80; i++ {
		days[i].ContributionCount, days[i].ContributionLevel = 1, 1
	}
	if _, err := TargetWindowAt(days, days[0].Date); err == nil {
		t.Fatal("one-target board accepted while an ideal board exists")
	}
	if window, err := TargetWindowAt(days, days[70].Date); err != nil || window.TargetCount != 10 {
		t.Fatal(window, err)
	}

	sparse := targetDays(12, 0)
	sparse[0].ContributionCount, sparse[0].ContributionLevel = 1, 1
	if _, err := TargetWindowAt(sparse, sparse[0].Date); err != nil {
		t.Fatal("closest sparse fallback should remain playable", err)
	}
}

func TestFairOpponentFallsBackToClosestPlayableWindow(t *testing.T) {
	days := targetDays(30, 0)
	for i := 0; i < 10; i++ {
		days[i].ContributionCount, days[i].ContributionLevel = 1, 1
	}
	for i := 70; i < 90; i++ {
		days[i].ContributionCount, days[i].ContributionLevel = 1, 1
	}
	for i := 140; i < 185; i++ {
		days[i].ContributionCount, days[i].ContributionLevel = 1, 1
	}
	player, _ := TargetWindowAt(days, days[0].Date)
	enemy, err := SelectFairOpponentWindow(days, player, fixed(0))
	if err != nil || enemy.StartIndex == player.StartIndex || enemy.TargetCount < 10 {
		t.Fatal(enemy, err)
	}
}

func TestNextTargetUsesOnlyPriorShotsAndNeverRepeats(t *testing.T) {
	shots := []TargetShot{}
	seen := map[Coord]bool{}
	for i := 0; i < BoardCells; i++ {
		coord, err := NextTarget(shots, fixed(0))
		if err != nil || seen[coord] {
			t.Fatal(coord, err)
		}
		seen[coord] = true
		shots = append(shots, TargetShot{Coord: coord, Result: "miss"})
	}
	if _, err := NextTarget(shots, fixed(0)); err == nil {
		t.Fatal("AI found a repeated coordinate after exhausting the board")
	}
}

func TestResolveTargetShotRejectsCorruptPriorResults(t *testing.T) {
	board := targetDays(10, 0)
	board[0].ContributionCount, board[0].ContributionLevel = 3, 2
	bad := []TargetShot{{Coord: Coord{X: 0, Y: 0}, Result: "miss"}}
	if _, _, err := ResolveTargetShot(board, bad, Coord{X: 0, Y: 1}); err == nil {
		t.Fatal("prior result inconsistent with frozen board was trusted")
	}
}
