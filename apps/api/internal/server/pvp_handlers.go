package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func (s *Server) pvpRepo(w http.ResponseWriter) (PVPRepository, bool) {
	p, ok := s.repo.(PVPRepository)
	if !ok {
		writeError(w, 501, "pvp_unavailable", "PvP is unavailable.")
	}
	return p, ok
}
func challengeDTO(c Challenge, u *User) map[string]any {
	creator := pvpUserDTO(c.Creator)
	creator["pvp"] = c.CreatorPVP
	d := map[string]any{"code": c.Code, "status": c.Status, "expiresAt": c.ExpiresAt, "creator": creator, "creatorReady": c.CreatorReady, "opponentReady": c.OpponentReady, "gameId": c.GameID, "canAccept": c.Status == "open"}
	if c.Opponent != nil {
		op := pvpUserDTO(*c.Opponent)
		op["pvp"] = c.OpponentPVP
		d["opponent"] = op
	}
	if u != nil {
		d["isCreator"] = u.ID == c.Creator.ID
		d["isOpponent"] = c.Opponent != nil && u.ID == c.Opponent.ID
		d["canCancel"] = u.ID == c.Creator.ID && c.Status == "open"
		d["canAccept"] = u.ID != c.Creator.ID && c.Status == "open" && (c.IntendedOpponentID == "" || c.IntendedOpponentID == u.ID)
	}
	return d
}
func pvpUserDTO(u User) map[string]any {
	return map[string]any{"login": u.Login, "name": u.Name, "avatarUrl": u.AvatarURL}
}
func pvpPlayerDTO(p PVPPlayer) map[string]any {
	u := pvpUserDTO(p.User)
	u["pvp"] = p.Stats
	return map[string]any{"user": u, "board": p.Board, "fleet": p.Fleet, "shots": p.Shots, "ready": p.Ready}
}
func pvpDTO(g *PVPGame) map[string]any {
	turn, start, winner := "", "", ""
	if g.CurrentTurn != "" {
		if g.CurrentTurn == g.You.User.ID {
			turn = "you"
		} else {
			turn = "opponent"
		}
	}
	if g.StartingPlayer != "" {
		if g.StartingPlayer == g.You.User.ID {
			start = "you"
		} else {
			start = "opponent"
		}
	}
	if g.Winner != "" {
		if g.Winner == g.You.User.ID {
			winner = "you"
		} else {
			winner = "opponent"
		}
	}
	d := map[string]any{"id": g.ID, "mode": "pvp", "status": g.Status, "currentTurn": turn, "startingPlayer": start, "winner": winner, "shareId": g.ShareID, "challengeCode": g.ChallengeCode, "updatedAt": g.UpdatedAt, "you": pvpPlayerDTO(g.You), "opponent": pvpPlayerDTO(g.Opponent), "yourTurn": g.CurrentTurn == g.You.User.ID, "result": map[string]any{"won": g.Winner == g.You.User.ID, "shots": len(g.You.Shots), "hits": g.Hits, "shipsSunk": g.ShipsSunk, "ratingBefore": g.RatingBefore, "ratingAfter": g.RatingAfter, "ratingDelta": g.RatingDelta, "rank": decorate(PublicStats{Rating: g.RatingAfter}).Rank, "opponentRatingBefore": g.OpponentRatingBefore, "opponentRatingAfter": g.OpponentRatingAfter, "opponentRatingDelta": g.OpponentRatingDelta, "opponentRank": decorate(PublicStats{Rating: g.OpponentRatingAfter}).Rank}}
	if g.LastMove != nil {
		by := "opponent"
		if g.LastMove.ShooterID == g.You.User.ID {
			by = "you"
		}
		d["lastMove"] = map[string]any{"by": by, "x": g.LastMove.Shot.X, "y": g.LastMove.Shot.Y, "result": g.LastMove.Shot.Result, "ship": g.LastMove.Shot.Ship}
	}
	return d
}
func (s *Server) publicChallenge(w http.ResponseWriter, r *http.Request) {
	p, ok := s.pvpRepo(w)
	if !ok {
		return
	}
	c, e := p.PublicChallenge(r.Context(), r.PathValue("code"))
	if e != nil {
		writeError(w, 404, "challenge_not_found", "Challenge was not found.")
		return
	}
	if c.Status == "expired" {
		writeError(w, 410, "challenge_expired", "This challenge has expired.")
		return
	}
	var current *User
	if bearer(r) != "" {
		u, err := s.currentUser(r)
		if err == nil {
			current = &u
		}
	}
	writeJSON(w, 200, challengeDTO(c, current))
}
func (s *Server) createChallenge(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.pvpRepo(w)
	if !ok {
		return
	}
	var c Challenge
	var e error
	for i := 0; i < 4; i++ {
		c, e = p.CreateChallenge(r.Context(), u.ID, randomSecret(12), s.now().Add(7*24*time.Hour))
		if e == nil {
			break
		}
	}
	if e != nil {
		writeError(w, 500, "challenge_create_failed", "Could not create challenge.")
		return
	}
	writeJSON(w, 201, map[string]any{"challenge": challengeDTO(c, &u), "challengeUrl": strings.TrimSuffix(s.cfg.WebAppURL, "/") + "/challenge/" + c.Code})
}
func (s *Server) acceptChallenge(w http.ResponseWriter, r *http.Request) {
	s.challengeAction(w, r, "accept")
}
func (s *Server) cancelChallenge(w http.ResponseWriter, r *http.Request) {
	s.challengeAction(w, r, "cancel")
}
func (s *Server) challengeAction(w http.ResponseWriter, r *http.Request, action string) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.pvpRepo(w)
	if !ok {
		return
	}
	var c Challenge
	var g *PVPGame
	var e error
	if action == "accept" {
		c, g, e = p.AcceptChallenge(r.Context(), u.ID, r.PathValue("code"), s.now())
	} else {
		c, e = p.CancelChallenge(r.Context(), u.ID, r.PathValue("code"), s.now())
	}
	if e != nil {
		if errors.Is(e, ErrConflict) {
			if action == "accept" {
				writeError(w, http.StatusConflict, "challenge_taken", "This challenge is no longer available to accept.")
			} else {
				writeError(w, http.StatusConflict, "challenge_not_open", "Only an open challenge can be cancelled.")
			}
			return
		}
		writePVPError(w, e)
		return
	}
	d := map[string]any{"challenge": challengeDTO(c, &u)}
	if g != nil {
		d["game"] = pvpDTO(g)
	}
	writeJSON(w, 200, d)
}
func (s *Server) readyChallenge(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.pvpRepo(w)
	if !ok {
		return
	}
	var q createReq
	if json.NewDecoder(r.Body).Decode(&q) != nil || game.ValidateFleet(q.Fleet) != nil {
		writeError(w, 422, "invalid_fleet", "Place every ship in bounds without overlap.")
		return
	}
	days, e := s.repo.Contributions(r.Context(), u.ID)
	if e != nil {
		writeError(w, 422, "invalid_period", "Contribution history is unavailable.")
		return
	}
	idx := -1
	for i := range days {
		if days[i].Date == q.StartDate {
			idx = i
			break
		}
	}
	if idx < 0 || idx%7 != 0 || idx+70 > len(days) {
		writeError(w, 422, "invalid_period", "Choose a complete 10-week range.")
		return
	}
	board := append([]game.Cell(nil), days[idx:idx+70]...)
	c, g, e := p.ReadyChallenge(r.Context(), u.ID, r.PathValue("code"), q.StartDate, board, q.Fleet, s.now())
	if e != nil {
		writePVPError(w, e)
		return
	}
	d := map[string]any{"challenge": challengeDTO(c, &u)}
	if g != nil {
		d["game"] = pvpDTO(g)
	}
	writeJSON(w, 200, d)
}
func (s *Server) battles(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.pvpRepo(w)
	if !ok {
		return
	}
	v, e := p.Battles(r.Context(), u.ID)
	if e != nil {
		writeError(w, 500, "battles_failed", "Could not load battles.")
		return
	}
	your, wait, done := []map[string]any{}, []map[string]any{}, []map[string]any{}
	for _, x := range v {
		winner := ""
		if x.Winner != "" {
			if x.Winner == u.ID {
				winner = "you"
			} else {
				winner = "opponent"
			}
		}
		d := map[string]any{"id": x.ID, "status": x.Status, "opponent": pvpUserDTO(x.Opponent), "yourTurn": x.YourTurn, "winner": winner, "challengeCode": x.ChallengeCode, "updatedAt": x.UpdatedAt}
		if x.Status == "complete" {
			done = append(done, d)
		} else if x.YourTurn {
			your = append(your, d)
		} else {
			wait = append(wait, d)
		}
	}
	writeJSON(w, 200, map[string]any{"yourTurn": your, "waiting": wait, "finished": done})
}
func (s *Server) rematch(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.pvpRepo(w)
	if !ok {
		return
	}
	c, e := p.Rematch(r.Context(), u.ID, r.PathValue("id"), s.now().Add(7*24*time.Hour))
	if e != nil {
		writePVPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"challenge": challengeDTO(c, &u), "challengeUrl": strings.TrimSuffix(s.cfg.WebAppURL, "/") + "/challenge/" + c.Code})
}
func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	p, ok := s.pvpRepo(w)
	if !ok {
		return
	}
	v, e := p.Leaderboard(r.Context(), 50)
	if e != nil {
		writeError(w, 500, "leaderboard_failed", "Could not load leaderboard.")
		return
	}
	writeJSON(w, 200, map[string]any{"entries": v})
}
func writePVPError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, ErrNotFound):
		writeError(w, 404, "challenge_not_found", "Challenge or game was not found.")
	case errors.Is(e, ErrExpired):
		writeError(w, 410, "challenge_expired", "This challenge has expired.")
	case errors.Is(e, ErrSelfChallenge):
		writeError(w, 409, "self_challenge", "You cannot accept your own challenge.")
	case errors.Is(e, ErrSetupLocked):
		writeError(w, 409, "setup_locked", "Your harbour is already locked.")
	case errors.Is(e, ErrNotYourTurn):
		writeError(w, 409, "not_your_turn", "Wait for your opponent's move.")
	case errors.Is(e, ErrGameComplete):
		writeError(w, 409, "game_complete", "This battle is already complete.")
	case e.Error() == "duplicate shot":
		writeError(w, 409, "duplicate_shot", "That coordinate has already been targeted.")
	case e.Error() == "shot out of bounds":
		writeError(w, 422, "invalid_coordinate", "Choose a coordinate inside the harbour grid.")
	case errors.Is(e, ErrConflict):
		writeError(w, 409, "action_unavailable", "This action is no longer available.")
	default:
		writeError(w, 409, "action_unavailable", "This action is not available.")
	}
}
