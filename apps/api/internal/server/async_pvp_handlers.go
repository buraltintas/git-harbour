package server

import (
	"encoding/json"
	"net/http"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func (s *Server) asyncPVPRepo(w http.ResponseWriter) (AsyncPVPRepository, bool) {
	p, ok := s.repo.(AsyncPVPRepository)
	if !ok {
		writeError(w, 501, "pvp_unavailable", "PvP is unavailable.")
	}
	return p, ok
}
func harbourDTO(h OpenHarbour, own bool) map[string]any {
	d := map[string]any{"owner": pvpUserDTO(h.Owner), "periodStart": h.PeriodStart, "fleetCapacity": h.Capacity, "updatedAt": h.UpdatedAt}
	if own {
		d["cells"] = h.Board
		d["deployment"] = h.Deployment
	}
	return d
}

func (s *Server) getOpenHarbour(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	h, e := p.OpenHarbour(r.Context(), u.ID)
	if e == ErrNotFound {
		writeJSON(w, 200, map[string]any{"open": false})
		return
	}
	if e != nil {
		writeError(w, 500, "harbour_failed", "Could not load your PvP harbour.")
		return
	}
	writeJSON(w, 200, map[string]any{"open": true, "harbour": harbourDTO(h, true)})
}
func (s *Server) setOpenHarbour(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var req struct {
		PlayerStart string                  `json:"playerStart"`
		Units       []game.DeploymentChoice `json:"units"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.PlayerStart == "" {
		writeError(w, 400, "invalid_pvp_harbour", "Choose and deploy a ten-week harbour.")
		return
	}
	days, e := s.repo.Contributions(r.Context(), u.ID)
	if e != nil {
		writeError(w, 422, "contributions_missing", "Contribution history is unavailable.")
		return
	}
	window, e := game.FleetWindowAt(days, req.PlayerStart)
	if e != nil {
		writeError(w, 422, "invalid_pvp_harbour", "That ten-week harbour is invalid.")
		return
	}
	units, e := game.ValidateDeployment(window.Cells, req.Units)
	if e != nil {
		writeError(w, 422, "deployment_rejected", e.Error())
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	h, e := p.SetOpenHarbour(r.Context(), u.ID, req.PlayerStart, window.Cells, units, s.now())
	if e != nil {
		writeError(w, 500, "harbour_open_failed", "Could not open your harbour.")
		return
	}
	writeJSON(w, 200, map[string]any{"open": true, "harbour": harbourDTO(h, true)})
}
func (s *Server) closeOpenHarbour(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	if e := p.CloseOpenHarbour(r.Context(), u.ID); e != nil && e != ErrNotFound {
		writeError(w, 500, "harbour_close_failed", "Could not close your harbour.")
		return
	}
	writeJSON(w, 200, map[string]bool{"open": false})
}
func (s *Server) listOpenHarbours(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	v, e := p.OpenHarbours(r.Context(), u.ID)
	if e != nil {
		writeError(w, 500, "harbours_failed", "Could not load open harbours.")
		return
	}
	out := make([]map[string]any, 0, len(v))
	for _, h := range v {
		out = append(out, harbourDTO(h, false))
	}
	writeJSON(w, 200, map[string]any{"harbours": out})
}
func (s *Server) startAsyncPVP(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var req struct {
		OpponentLogin string `json:"opponentLogin"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.OpponentLogin == "" {
		writeError(w, 400, "opponent_required", "Choose an open harbour.")
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	b, e := p.StartAsyncPVP(r.Context(), u.ID, req.OpponentLogin, s.now())
	if e != nil {
		writeAsyncPVPError(w, e)
		return
	}
	writeJSON(w, 201, asyncPVPDTO(b, u.ID))
}
func (s *Server) listAsyncPVP(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	battles, e := p.AsyncPVPBattles(r.Context(), u.ID)
	if e != nil {
		writeError(w, 500, "battles_failed", "Could not load your PvP battles.")
		return
	}
	out := make([]map[string]any, 0, len(battles))
	for _, b := range battles {
		out = append(out, asyncPVPSummaryDTO(b, u.ID))
	}
	writeJSON(w, 200, map[string]any{"battles": out})
}

func asyncPVPSummaryDTO(b *AsyncPVPBattle, uid string) map[string]any {
	role, opponent := "challenger", b.Defender
	myAlive, oppAlive := game.AliveCount(b.State.PlayerDeployment), game.AliveCount(b.State.EnemyDeployment)
	if uid == b.Defender.ID {
		role, opponent = "defender", b.Challenger
		myAlive, oppAlive = oppAlive, myAlive
	}
	complete := b.State.Status == "complete"
	d := map[string]any{
		"id": b.State.ID, "status": b.State.Status, "role": role,
		"opponent": pvpUserDTO(opponent), "updatedAt": b.UpdatedAt,
		"yourTurn":           role == "challenger" && !complete,
		"myUnitsAlive":       myAlive,
		"opponentUnitsAlive": oppAlive,
	}
	if complete {
		youWon := (role == "challenger" && b.State.Winner == "player") || (role == "defender" && b.State.Winner == "ai")
		if youWon {
			d["winner"] = "you"
		} else {
			d["winner"] = "opponent"
		}
	}
	return d
}

func (s *Server) getAsyncPVP(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	b, e := p.AsyncPVPGame(r.Context(), u.ID, r.PathValue("id"))
	if e != nil {
		writeAsyncPVPError(w, e)
		return
	}
	writeJSON(w, 200, asyncPVPDTO(b, u.ID))
}
func (s *Server) shootAsyncPVP(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Target game.Coord `json:"target"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "invalid_coordinate", "Choose an enemy coordinate.")
		return
	}
	p, ok := s.asyncPVPRepo(w)
	if !ok {
		return
	}
	b, events, e := p.ShootAsyncPVP(r.Context(), u.ID, r.PathValue("id"), req.Target)
	if e != nil {
		writeAsyncPVPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"game": asyncPVPDTO(b, u.ID), "events": events})
}

func asyncPVPDTO(b *AsyncPVPBattle, uid string) map[string]any {
	if uid != b.Challenger.ID && b.State.Status != "complete" {
		return map[string]any{"id": b.State.ID, "mode": "pvp", "ruleset": b.State.Ruleset, "status": b.State.Status, "role": "defender", "automatedDefense": true, "challenger": pvpUserDTO(b.Challenger), "defender": pvpUserDTO(b.Defender), "playerUnitsAlive": game.AliveCount(b.State.PlayerDeployment), "enemyUnitsAlive": game.AliveCount(b.State.EnemyDeployment), "updatedAt": b.UpdatedAt}
	}
	d := publicGame(b.State)
	d["mode"] = "pvp"
	d["role"] = "challenger"
	if uid == b.Defender.ID {
		d["role"] = "defender"
	}
	d["automatedDefense"] = true
	d["challenger"] = pvpUserDTO(b.Challenger)
	d["defender"] = pvpUserDTO(b.Defender)
	d["opponent"] = pvpUserDTO(b.Defender)
	d["updatedAt"] = b.UpdatedAt
	return d
}
func writeAsyncPVPError(w http.ResponseWriter, e error) {
	switch e {
	case ErrSetupLocked:
		writeError(w, 409, "open_your_harbour", "Open and deploy your own harbour first.")
	case ErrSelfChallenge:
		writeError(w, 409, "self_challenge", "Choose another developer.")
	case ErrNotFound:
		writeError(w, 404, "pvp_not_found", "The open harbour or battle was not found.")
	case ErrConflict:
		writeError(w, 409, "harbour_not_open", "That developer's harbour is no longer open.")
	case ErrGameComplete:
		writeError(w, 409, "game_complete", "This battle is complete.")
	default:
		if e.Error() == "duplicate shot" {
			writeError(w, 409, "duplicate_shot", "That coordinate was already targeted.")
			return
		}
		writeError(w, 409, "pvp_action_failed", e.Error())
	}
}
