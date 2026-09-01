package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
var ErrExpired = errors.New("expired or consumed")
var ErrConflict = errors.New("conflict")
var ErrSelfChallenge = errors.New("self challenge")
var ErrNotYourTurn = errors.New("not your turn")
var ErrSetupLocked = errors.New("setup locked")
var ErrGameComplete = errors.New("game complete")
var ErrLegacyGame = errors.New("legacy game retired")

func randomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func digest(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }
func writeJSON(w http.ResponseWriter, n int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(n)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, n int, code, msg string) {
	writeJSON(w, n, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
func decorate(p PublicStats) PublicStats {
	if p.Rating == 0 {
		p.Rating = 1200
	}
	if p.Games > 0 {
		p.WinRate = 100 * float64(p.Wins) / float64(p.Games)
	}
	if p.Shots > 0 {
		p.Accuracy = 100 * float64(p.Hits) / float64(p.Shots)
	}
	if p.Wins > 0 {
		p.AverageShotsPerWin = float64(p.WinShots) / float64(p.Wins)
	}
	switch {
	case p.Rating < 900:
		p.Rank = "Deckhand"
	case p.Rating < 1100:
		p.Rank = "Sailor"
	case p.Rating < 1300:
		p.Rank = "Officer"
	case p.Rating < 1500:
		p.Rank = "Commander"
	case p.Rating < 1700:
		p.Rank = "Captain"
	case p.Rating < 1900:
		p.Rank = "Admiral"
	default:
		p.Rank = "Fleet Admiral"
	}
	return p
}
func publicGame(g *State) map[string]any {
	if g.Ruleset == game.ContributionBattleshipRuleset {
		return publicBattleshipGame(g)
	}
	if g.Ruleset == game.ContributionFleetRuleset {
		return publicFleetGame(g)
	}
	if g.Ruleset == "contribution_targets_v2" {
		return publicTargetGame(g)
	}
	b, _ := json.Marshal(g)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	delete(m, "enemyFleet")
	delete(m, "enemyStart")
	if g.Status == "complete" {
		m["enemyPeriod"] = map[string]string{"start": g.EnemyStart, "end": dateEnd(g.EnemyStart)}
	}
	return m
}

func publicBattleshipGame(g *State) map[string]any {
	playerHits, playerMisses := targetShotCounts(g.PlayerTargetShots)
	aiHits, aiMisses := targetShotCounts(g.AITargetShots)
	reveal := g.Status == "complete"
	out := map[string]any{
		"id": g.ID, "mode": "solo", "ruleset": g.Ruleset, "status": g.Status,
		"currentTurn": g.Turn, "winner": g.Winner,
		"playerCells":         projectBattleshipBoard(g.PlayerBoard, g.PlayerDeployment, g.AITargetShots, true, reveal),
		"enemyCells":          projectBattleshipBoard(g.EnemyBoard, g.EnemyDeployment, g.PlayerTargetShots, false, reveal),
		"playerFleetCapacity": game.FleetCapacity(game.TargetCount(g.PlayerBoard)),
		"enemyFleetCapacity":  game.FleetCapacity(game.TargetCount(g.EnemyBoard)),
		"playerUnitsAlive":    game.AliveCount(g.PlayerDeployment), "enemyUnitsAlive": game.AliveCount(g.EnemyDeployment),
		"playerSummary": windowSummary(g.PlayerBoard), "stats": g.Stats,
		"turns": len(g.PlayerTargetShots), "shots": len(g.PlayerTargetShots), "hits": playerHits, "misses": playerMisses,
		"aiShots": len(g.AITargetShots), "aiHits": aiHits, "aiMisses": aiMisses,
		"playerPeriod": map[string]string{"start": g.PlayerStart, "end": dateEnd(g.PlayerStart)},
	}
	events := make([]game.BattleEvent, 0, len(g.PlayerTargetShots)+len(g.AITargetShots))
	for i := range g.PlayerTargetShots {
		events = append(events, game.BattleEvent{Actor: "player", TargetShot: g.PlayerTargetShots[i]})
		if i < len(g.AITargetShots) {
			events = append(events, game.BattleEvent{Actor: "ai", TargetShot: g.AITargetShots[i]})
		}
	}
	if start := len(events) - 6; start > 0 {
		events = events[start:]
	}
	out["recentShots"] = events
	if g.Status == "deployment" {
		out["deploymentRequired"] = len(g.PlayerDeployment) == 0
	}
	if reveal {
		out["enemyPeriod"] = map[string]string{"start": g.EnemyStart, "end": dateEnd(g.EnemyStart)}
		out["enemySummary"] = windowSummary(g.EnemyBoard)
		out["ratingDelta"], out["shareId"] = g.RatingDelta, g.ShareID
	}
	return out
}

func projectBattleshipBoard(board []game.Cell, units []game.FleetUnit, shots []game.TargetShot, own, reveal bool) []map[string]any {
	byCoord := map[game.Coord]game.FleetUnit{}
	for _, unit := range units {
		byCoord[unit.Coord] = unit
	}
	shotByCoord := map[game.Coord]game.TargetShot{}
	for _, shot := range shots {
		shotByCoord[shot.Coord] = shot
	}
	out := make([]map[string]any, 0, len(board))
	for i, cell := range board {
		coord := game.Coord{X: i / game.Height, Y: i % game.Height}
		projected := map[string]any{"x": coord.X, "y": coord.Y, "state": "unknown"}
		if own || reveal {
			projected["date"], projected["weekday"] = cell.Date, cell.Weekday
			projected["contributionCount"], projected["contributionLevel"] = cell.ContributionCount, cell.ContributionLevel
			projected["state"] = "empty"
			if cell.ContributionCount > 0 {
				projected["state"] = "eligible"
			}
		}
		if unit, ok := byCoord[coord]; ok && (own || reveal) {
			projected["state"] = "deployed"
			if !unit.Alive {
				projected["state"] = "eliminated"
			}
			projected["unitKind"] = unit.Kind
		}
		if shot, ok := shotByCoord[coord]; ok {
			projected["state"] = shot.Result
			if shot.Result == "hit" {
				projected["state"] = "eliminated"
			}
		}
		out = append(out, projected)
	}
	return out
}

func windowSummary(cells []game.Cell) map[string]any {
	w, _ := game.SummarizeFleetWindow(cells, 0)
	return map[string]any{
		"totalContributions": w.TotalContributions, "activeDays": w.ActiveDays,
		"contributionPower": w.ContributionPower, "fleetCapacity": w.FleetCapacity,
		"peakCount": w.PeakCount, "peakDate": w.PeakDate, "maxDeployedPower": w.MaxDeployedPower,
	}
}

func publicFleetGame(g *State) map[string]any {
	reveal := g.Status == "complete"
	playerActions, aiActions, playerMisses, aiMisses, playerClashes, aiClashes, playerWins, aiWins := fleetMetrics(g.FleetActions)
	out := map[string]any{
		"id": g.ID, "mode": "solo", "ruleset": g.Ruleset, "status": g.Status,
		"currentTurn": g.Turn, "winner": g.Winner, "playerCells": projectFleetBoard(g.PlayerBoard, g.PlayerDeployment, g.FleetActions, true, reveal),
		"enemyCells":          projectFleetBoard(g.EnemyBoard, g.EnemyDeployment, g.FleetActions, false, reveal),
		"playerFleetCapacity": game.FleetCapacity(game.TargetCount(g.PlayerBoard)),
		"enemyFleetCapacity":  game.FleetCapacity(game.TargetCount(g.EnemyBoard)),
		"playerUnitsAlive":    game.AliveCount(g.PlayerDeployment), "enemyUnitsAlive": game.AliveCount(g.EnemyDeployment),
		"playerSummary": windowSummary(g.PlayerBoard), "stats": g.Stats,
		"turns": playerActions, "shots": playerActions, "misses": playerMisses, "clashes": playerClashes,
		"clashesWon": playerWins, "clashesLost": playerClashes - playerWins,
		"aiShots": aiActions, "aiMisses": aiMisses, "aiClashes": aiClashes,
		"aiClashesWon": aiWins, "aiClashesLost": aiClashes - aiWins,
		"playerPeriod": map[string]string{"start": g.PlayerStart, "end": dateEnd(g.PlayerStart)},
	}
	actionStart := len(g.FleetActions) - 6
	if actionStart < 0 {
		actionStart = 0
	}
	out["recentActions"] = publicFleetEvents(g.FleetActions[actionStart:])
	if g.Status == "deployment" {
		out["deploymentRequired"] = len(g.PlayerDeployment) == 0
	}
	if reveal {
		out["enemyPeriod"] = map[string]string{"start": g.EnemyStart, "end": dateEnd(g.EnemyStart)}
		out["enemySummary"] = windowSummary(g.EnemyBoard)
		out["playerStartingPower"], out["enemyStartingPower"] = fleetPower(g.PlayerDeployment, false), fleetPower(g.EnemyDeployment, false)
		out["playerSurvivingPower"], out["enemySurvivingPower"] = fleetPower(g.PlayerDeployment, true), fleetPower(g.EnemyDeployment, true)
		out["playerStrongestUnit"], out["enemyStrongestUnit"] = strongestUnit(g.PlayerBoard, g.PlayerDeployment), strongestUnit(g.EnemyBoard, g.EnemyDeployment)
		playerUpsets, aiUpsets := 0, 0
		for _, action := range g.FleetActions {
			if action.Result == "clash" && action.AttackerWon && action.AttackerPower < action.DefenderPower {
				if action.Actor == "player" {
					playerUpsets++
				} else {
					aiUpsets++
				}
			}
		}
		out["playerUpsetWins"], out["aiUpsetWins"] = playerUpsets, aiUpsets
		out["ratingDelta"], out["shareId"] = g.RatingDelta, g.ShareID
		out["actions"] = g.FleetActions
	}
	return out
}

func fleetPower(units []game.FleetUnit, survivingOnly bool) float64 {
	total := 0.0
	for _, unit := range units {
		if !survivingOnly || unit.Alive {
			total += unit.Power
		}
	}
	return total
}
func strongestUnit(board []game.Cell, units []game.FleetUnit) map[string]any {
	var best *game.FleetUnit
	for i := range units {
		if best == nil || units[i].Power > best.Power {
			best = &units[i]
		}
	}
	if best == nil {
		return nil
	}
	out := map[string]any{"kind": best.Kind, "power": best.Power, "combatLevel": best.Level}
	if best.Kind == "contribution" {
		cell := board[best.X*game.Height+best.Y]
		out["date"], out["contributionCount"] = cell.Date, cell.ContributionCount
	}
	return out
}

func fleetMetrics(actions []game.FleetAction) (playerActions, aiActions, playerMisses, aiMisses, playerClashes, aiClashes, playerWins, aiWins int) {
	for _, action := range actions {
		if action.Actor == "player" {
			playerActions++
			if action.Result == "miss" {
				playerMisses++
			} else {
				playerClashes++
				if action.AttackerWon {
					playerWins++
				}
			}
		} else {
			aiActions++
			if action.Result == "miss" {
				aiMisses++
			} else {
				aiClashes++
				if action.AttackerWon {
					aiWins++
				}
			}
		}
	}
	return
}

func projectFleetBoard(board []game.Cell, units []game.FleetUnit, actions []game.FleetAction, own, reveal bool) []map[string]any {
	byCoord := map[game.Coord]game.FleetUnit{}
	for _, unit := range units {
		byCoord[unit.Coord] = unit
	}
	targeted := map[game.Coord]bool{}
	missed := map[game.Coord]bool{}
	actor := "player"
	if own {
		actor = "ai"
	}
	for _, action := range actions {
		if action.Actor == actor {
			targeted[action.Target] = true
			if action.Result == "miss" {
				missed[action.Target] = true
			}
		}
	}
	out := make([]map[string]any, 0, len(board))
	for i, cell := range board {
		coord := game.Coord{X: i / game.Height, Y: i % game.Height}
		projected := map[string]any{"x": coord.X, "y": coord.Y, "state": "unknown"}
		if targeted[coord] {
			projected["targeted"] = true
		}
		unit, deployed := byCoord[coord]
		if deployed && unit.Alive && unit.Exposed {
			delete(projected, "targeted")
		}
		if own || reveal {
			projected["date"], projected["weekday"] = cell.Date, cell.Weekday
			projected["contributionCount"], projected["contributionLevel"] = cell.ContributionCount, cell.ContributionLevel
			projected["dayPower"], projected["combatLevel"] = game.DayPower(cell.ContributionCount), game.CombatLevel(game.DayPower(cell.ContributionCount))
			projected["state"] = "empty"
			if cell.ContributionCount > 0 {
				projected["state"] = "eligible"
			}
		}
		if missed[coord] {
			projected["state"] = "miss"
		}
		if deployed && (own || reveal || unit.Exposed) {
			state := "deployed"
			if unit.Exposed {
				state = "exposed"
			}
			if !unit.Alive {
				state = "eliminated"
			}
			projected["state"], projected["combatLevel"] = state, unit.Level
			if own || reveal {
				projected["unitKind"] = unit.Kind
				projected["unitPower"] = unit.Power
			}
		}
		out = append(out, projected)
	}
	return out
}

func publicFleetEvents(events []game.FleetAction) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, event := range events {
		out = append(out, map[string]any{"actor": event.Actor, "attacker": event.Attacker, "target": event.Target, "result": event.Result, "attackerWon": event.AttackerWon})
	}
	return out
}

func publicTargetGame(g *State) map[string]any {
	playerHits, playerMisses := targetShotCounts(g.PlayerTargetShots)
	aiHits, aiMisses := targetShotCounts(g.AITargetShots)
	playerCells := projectOwnBoard(g.PlayerBoard, g.AITargetShots)
	enemyCells := projectEnemyBoard(g.EnemyBoard, g.PlayerTargetShots, g.Status == "complete")
	accuracy := 0.0
	if len(g.PlayerTargetShots) > 0 {
		accuracy = 100 * float64(playerHits) / float64(len(g.PlayerTargetShots))
	}
	aiAccuracy := 0.0
	if len(g.AITargetShots) > 0 {
		aiAccuracy = 100 * float64(aiHits) / float64(len(g.AITargetShots))
	}
	out := map[string]any{
		"id": g.ID, "mode": "solo", "ruleset": g.Ruleset,
		"status": g.Status, "currentTurn": g.Turn, "winner": g.Winner,
		"playerCells": playerCells, "enemyCells": enemyCells,
		"playerTargetCount": g.PlayerTargetCount, "enemyTargetCount": g.EnemyTargetCount,
		"playerTargetsHit": aiHits, "enemyTargetsHit": playerHits,
		"shots": len(g.PlayerTargetShots), "misses": playerMisses,
		"accuracy": accuracy, "aiShots": len(g.AITargetShots), "aiMisses": aiMisses, "aiAccuracy": aiAccuracy, "stats": g.Stats,
		"playerPeriod": map[string]string{"start": g.PlayerStart, "end": dateEnd(g.PlayerStart)},
	}
	if g.Status == "complete" {
		out["enemyPeriod"] = map[string]string{"start": g.EnemyStart, "end": dateEnd(g.EnemyStart)}
		out["ratingDelta"] = g.RatingDelta
		out["shareId"] = g.ShareID
	}
	return out
}

func targetShotCounts(shots []game.TargetShot) (hits, misses int) {
	for _, shot := range shots {
		if shot.Result == "hit" {
			hits++
		} else {
			misses++
		}
	}
	return
}

func projectOwnBoard(board []game.Cell, shots []game.TargetShot) []map[string]any {
	shotByCoord := map[string]game.TargetShot{}
	for _, shot := range shots {
		shotByCoord[fmt.Sprintf("%d:%d", shot.X, shot.Y)] = shot
	}
	out := make([]map[string]any, 0, len(board))
	for i, cell := range board {
		x, y := i/game.Height, i%game.Height
		state := "empty"
		if cell.ContributionCount > 0 {
			state = "target"
		}
		if shot, ok := shotByCoord[fmt.Sprintf("%d:%d", x, y)]; ok {
			state = shot.Result
		}
		out = append(out, map[string]any{"x": x, "y": y, "state": state, "date": cell.Date, "weekday": cell.Weekday, "contributionCount": cell.ContributionCount, "contributionLevel": cell.ContributionLevel})
	}
	return out
}

func projectEnemyBoard(board []game.Cell, shots []game.TargetShot, reveal bool) []map[string]any {
	shotByCoord := map[string]game.TargetShot{}
	for _, shot := range shots {
		shotByCoord[fmt.Sprintf("%d:%d", shot.X, shot.Y)] = shot
	}
	out := make([]map[string]any, 0, len(board))
	for i, cell := range board {
		x, y := i/game.Height, i%game.Height
		projected := map[string]any{"x": x, "y": y, "state": "unknown"}
		if reveal {
			projected["date"], projected["weekday"] = cell.Date, cell.Weekday
			projected["contributionCount"], projected["contributionLevel"] = cell.ContributionCount, cell.ContributionLevel
			if cell.ContributionCount == 0 {
				projected["state"] = "empty"
			} else {
				projected["state"] = "target"
			}
		}
		if shot, ok := shotByCoord[fmt.Sprintf("%d:%d", x, y)]; ok {
			projected["state"] = shot.Result
			if shot.Result == "hit" {
				projected["contributionCount"] = shot.ContributionCount
				projected["contributionLevel"] = shot.ContributionLevel
			}
		}
		out = append(out, projected)
	}
	return out
}
func dateEnd(v string) string {
	d, _ := time.Parse("2006-01-02", v)
	return d.AddDate(0, 0, 69).Format("2006-01-02")
}
func stripDates(c []game.Cell) []game.Cell {
	o := append([]game.Cell(nil), c...)
	for i := range o {
		o[i].Date = ""
	}
	return o
}
func countHits(v []game.Shot) int {
	n := 0
	for _, s := range v {
		if s.Result != "miss" {
			n++
		}
	}
	return n
}
func finishState(g *State, p PublicStats, winner string) (PublicStats, string) {
	if g.TerminalApplied {
		return p, g.ShareID
	}
	g.Status = "complete"
	g.Turn = "complete"
	g.Winner = winner
	g.TerminalApplied = true
	old := p.Rating
	gs := game.Stats{Games: p.Games, Wins: p.Wins, Losses: p.Losses, Rating: p.Rating, Shots: p.Shots, Hits: p.Hits, CurrentStreak: p.CurrentStreak, LongestStreak: p.LongestStreak, WinShots: p.WinShots}
	gs = game.UpdateStats(gs, winner == "player", len(g.PlayerShots), countHits(g.PlayerShots))
	p = decorate(PublicStats{Games: gs.Games, Wins: gs.Wins, Losses: gs.Losses, Rating: gs.Rating, Shots: gs.Shots, Hits: gs.Hits, CurrentStreak: gs.CurrentStreak, LongestStreak: gs.LongestStreak, WinShots: gs.WinShots})
	g.Stats = p
	g.RatingDelta = p.Rating - old
	g.ShareID = randomSecret(9)
	return p, g.ShareID
}
func finishTargetState(g *State, p PublicStats, winner string) PublicStats {
	if g.TerminalApplied {
		return p
	}
	g.Status, g.Turn, g.Winner, g.TerminalApplied = "complete", "complete", winner, true
	old := p.Rating
	hits, _ := targetShotCounts(g.PlayerTargetShots)
	stats := game.Stats{Games: p.Games, Wins: p.Wins, Losses: p.Losses, Rating: p.Rating, Shots: p.Shots, Hits: p.Hits, CurrentStreak: p.CurrentStreak, LongestStreak: p.LongestStreak, WinShots: p.WinShots}
	stats = game.UpdateStatsAgainst(stats, 1200, winner == "player", len(g.PlayerTargetShots), hits)
	p = decorate(PublicStats{Games: stats.Games, Wins: stats.Wins, Losses: stats.Losses, Rating: stats.Rating, Shots: stats.Shots, Hits: stats.Hits, CurrentStreak: stats.CurrentStreak, LongestStreak: stats.LongestStreak, WinShots: stats.WinShots})
	g.Stats, g.RatingDelta, g.ShareID = p, p.Rating-old, randomSecret(9)
	return p
}
func finishFleetState(g *State, p PublicStats, winner string) PublicStats {
	if g.TerminalApplied {
		return p
	}
	g.Status, g.Turn, g.Winner, g.TerminalApplied = "complete", "complete", winner, true
	old := p.Rating
	playerActions, _, _, _, playerClashes, _, _, _ := fleetMetrics(g.FleetActions)
	stats := game.Stats{Games: p.Games, Wins: p.Wins, Losses: p.Losses, Rating: p.Rating, Shots: p.Shots, Hits: p.Hits, CurrentStreak: p.CurrentStreak, LongestStreak: p.LongestStreak, WinShots: p.WinShots}
	stats = game.UpdateStatsAgainst(stats, 1200, winner == "player", playerActions, playerClashes)
	p = decorate(PublicStats{Games: stats.Games, Wins: stats.Wins, Losses: stats.Losses, Rating: stats.Rating, Shots: stats.Shots, Hits: stats.Hits, CurrentStreak: stats.CurrentStreak, LongestStreak: stats.LongestStreak, WinShots: stats.WinShots})
	g.Stats, g.RatingDelta, g.ShareID = p, p.Rating-old, randomSecret(9)
	return p
}

func resolveFleetTurn(g *State, p PublicStats, attacker, target game.Coord, r game.Rander) ([]game.FleetAction, PublicStats, error) {
	if g.Ruleset != game.ContributionFleetRuleset || len(g.PlayerBoard) != game.BoardCells || len(g.EnemyBoard) != game.BoardCells {
		return nil, p, ErrLegacyGame
	}
	if g.Status == "complete" {
		return nil, p, ErrGameComplete
	}
	if g.Status != "battle" || g.Turn != "player" {
		return nil, p, ErrNotYourTurn
	}
	event, player, enemy, err := game.ResolveFleetAction(g.PlayerDeployment, g.EnemyDeployment, g.FleetActions, "player", attacker, target, r)
	if err != nil {
		return nil, p, err
	}
	g.PlayerDeployment, g.EnemyDeployment = player, enemy
	g.FleetActions = append(g.FleetActions, event)
	events := []game.FleetAction{event}
	if game.AliveCount(g.EnemyDeployment) == 0 {
		p = finishFleetState(g, p, "player")
		return events, p, nil
	}
	if game.AliveCount(g.PlayerDeployment) == 0 {
		p = finishFleetState(g, p, "ai")
		return events, p, nil
	}
	g.Turn = "ai"
	aiAttacker, aiTarget, err := game.NextComputerAction(g.EnemyDeployment, g.FleetActions, r)
	if err != nil {
		return nil, p, err
	}
	aiEvent, enemy, player, err := game.ResolveFleetAction(g.EnemyDeployment, g.PlayerDeployment, g.FleetActions, "ai", aiAttacker, aiTarget, r)
	if err != nil {
		return nil, p, err
	}
	g.EnemyDeployment, g.PlayerDeployment = enemy, player
	g.FleetActions = append(g.FleetActions, aiEvent)
	events = append(events, aiEvent)
	if game.AliveCount(g.PlayerDeployment) == 0 {
		p = finishFleetState(g, p, "ai")
	} else if game.AliveCount(g.EnemyDeployment) == 0 {
		p = finishFleetState(g, p, "player")
	} else {
		g.Turn = "player"
	}
	return events, p, nil
}
func resolveTargetTurn(g *State, p PublicStats, c game.Coord, r game.Rander) ([]game.BattleEvent, PublicStats, error) {
	if g.Ruleset != "contribution_targets_v2" {
		return nil, p, ErrLegacyGame
	}
	if len(g.PlayerBoard) != game.BoardCells || len(g.EnemyBoard) != game.BoardCells {
		return nil, p, ErrLegacyGame
	}
	if g.Status == "complete" {
		return nil, p, ErrGameComplete
	}
	if g.Turn != "player" {
		return nil, p, ErrNotYourTurn
	}
	shot, complete, err := game.ResolveTargetShot(g.EnemyBoard, g.PlayerTargetShots, c)
	if err != nil {
		return nil, p, err
	}
	g.PlayerTargetShots = append(g.PlayerTargetShots, shot)
	events := []game.BattleEvent{{Actor: "player", TargetShot: shot}}
	if complete {
		p = finishTargetState(g, p, "player")
		return events, p, nil
	}
	g.Turn = "ai"
	target, err := game.NextTarget(g.AITargetShots, r)
	if err != nil {
		return nil, p, err
	}
	aiShot, aiComplete, err := game.ResolveTargetShot(g.PlayerBoard, g.AITargetShots, target)
	if err != nil {
		return nil, p, err
	}
	g.AITargetShots = append(g.AITargetShots, aiShot)
	events = append(events, game.BattleEvent{Actor: "ai", TargetShot: aiShot})
	if aiComplete {
		p = finishTargetState(g, p, "ai")
	} else {
		g.Turn = "player"
	}
	return events, p, nil
}
func resolveBattleshipTurn(g *State, p PublicStats, c game.Coord, r game.Rander) ([]game.BattleEvent, PublicStats, error) {
	if g.Ruleset != game.ContributionBattleshipRuleset || len(g.PlayerBoard) != game.BoardCells || len(g.EnemyBoard) != game.BoardCells {
		return nil, p, ErrLegacyGame
	}
	if g.Status == "complete" {
		return nil, p, ErrGameComplete
	}
	if g.Status != "battle" || g.Turn != "player" {
		return nil, p, ErrNotYourTurn
	}
	shot, enemy, err := game.ResolveDeploymentShot(g.EnemyDeployment, g.PlayerTargetShots, c)
	if err != nil {
		return nil, p, err
	}
	g.EnemyDeployment = enemy
	g.PlayerTargetShots = append(g.PlayerTargetShots, shot)
	events := []game.BattleEvent{{Actor: "player", TargetShot: shot}}
	if game.AliveCount(g.EnemyDeployment) == 0 {
		return events, finishTargetState(g, p, "player"), nil
	}
	g.Turn = "ai"
	target, err := game.NextTarget(g.AITargetShots, r)
	if err != nil {
		return nil, p, err
	}
	aiShot, player, err := game.ResolveDeploymentShot(g.PlayerDeployment, g.AITargetShots, target)
	if err != nil {
		return nil, p, err
	}
	g.PlayerDeployment = player
	g.AITargetShots = append(g.AITargetShots, aiShot)
	events = append(events, game.BattleEvent{Actor: "ai", TargetShot: aiShot})
	if game.AliveCount(g.PlayerDeployment) == 0 {
		p = finishTargetState(g, p, "ai")
	} else {
		g.Turn = "player"
	}
	return events, p, nil
}
func resolveTurn(g *State, p PublicStats, c game.Coord) ([]game.Shot, PublicStats, error) {
	if g.Status == "complete" {
		return nil, p, errors.New("game is complete")
	}
	if g.Turn != "player" {
		return nil, p, errors.New("wrong turn")
	}
	ev, n, e := game.ResolveShot(g.EnemyFleet, g.PlayerShots, c)
	if e != nil {
		return nil, p, e
	}
	g.EnemyFleet = n
	g.PlayerShots = append(g.PlayerShots, ev)
	events := []game.Shot{ev}
	if game.AllSunk(g.EnemyFleet) {
		p, _ = finishState(g, p, "player")
		return events, p, nil
	}
	g.Turn = "ai"
	target, e := game.NextAITarget(g.AIShots, game.SecureRand{})
	if e != nil {
		return nil, p, e
	}
	a, pf, e := game.ResolveShot(g.PlayerFleet, g.AIShots, target)
	if e != nil {
		return nil, p, e
	}
	g.PlayerFleet = pf
	g.AIShots = append(g.AIShots, a)
	events = append(events, a)
	if game.AllSunk(g.PlayerFleet) {
		p, _ = finishState(g, p, "ai")
	} else {
		g.Turn = "player"
	}
	return events, p, nil
}
func escape(v string) string { return html.EscapeString(v) }
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}
