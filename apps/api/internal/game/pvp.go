package game

import "errors"

const PVPStatusBattle = "battle"

// PVPTransition is the complete server-derived result of one PvP shot. The
// caller persists TargetFleet and Shot, then either changes the turn to
// NextTurnID or completes the game with WinnerID.
type PVPTransition struct {
	Shot        Shot
	TargetFleet []Ship
	NextTurnID  string
	WinnerID    string
}

// ChooseStartingPlayer chooses one of two distinct participants using the
// supplied random source. Production callers should pass SecureRand.
func ChooseStartingPlayer(players [2]string, r Rander) (string, error) {
	if players[0] == "" || players[1] == "" || players[0] == players[1] {
		return "", errors.New("two distinct players are required")
	}
	if r == nil {
		return "", errors.New("random source is required")
	}
	n, err := r.Intn(len(players))
	if err != nil {
		return "", err
	}
	if n < 0 || n >= len(players) {
		return "", errors.New("random source returned an invalid player index")
	}
	return players[n], nil
}

// ResolvePVPTurn validates and resolves one asynchronous PvP turn. It accepts
// only player intent (the target coordinate) and derives hit/miss/sunk,
// victory, and the next player. Fleet validation belongs to setup; this
// transition deliberately reuses the shared shot and victory rules.
func ResolvePVPTurn(
	status string,
	currentTurnID string,
	shooterID string,
	opponentID string,
	targetFleet []Ship,
	previousShots []Shot,
	target Coord,
) (PVPTransition, error) {
	if status != PVPStatusBattle {
		return PVPTransition{}, errors.New("game is not in battle")
	}
	if shooterID == "" || opponentID == "" || shooterID == opponentID {
		return PVPTransition{}, errors.New("two distinct players are required")
	}
	if currentTurnID != shooterID {
		return PVPTransition{}, errors.New("wrong turn")
	}
	if err := ValidateFleet(targetFleet); err != nil {
		return PVPTransition{}, err
	}

	shot, nextFleet, err := ResolveShot(targetFleet, previousShots, target)
	if err != nil {
		return PVPTransition{}, err
	}
	transition := PVPTransition{Shot: shot, TargetFleet: nextFleet}
	if AllSunk(nextFleet) {
		transition.WinnerID = shooterID
		return transition, nil
	}
	transition.NextTurnID = opponentID
	return transition, nil
}

// UpdateStatsAgainst applies a completed match using the opponent's rating
// from before the match. Callers completing PvP must calculate both players'
// results from their two unchanged pre-match ratings before persisting either.
func UpdateStatsAgainst(s Stats, opponentRating int, won bool, shots, hits int) Stats {
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
	s.Rating = Elo(s.Rating, opponentRating, won)
	return s
}
