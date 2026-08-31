package server

import (
	"encoding/json"
	"net/http"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	solo, e := s.repo.Stats(r.Context(), u.ID, "solo")
	if e != nil {
		writeError(w, 500, "stats_failed", "Could not load stats.")
		return
	}
	pvp, e := s.repo.Stats(r.Context(), u.ID, "pvp")
	if e != nil {
		writeError(w, 500, "stats_failed", "Could not load stats.")
		return
	}
	d := userDTO(u)
	d["solo"] = solo
	d["pvp"] = pvp
	d["publicProfileUrl"] = s.cfg.PublicAPIURL + "/u/" + u.Login
	writeJSON(w, 200, d)
}
func (s *Server) contributions(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	days, e := s.repo.Contributions(r.Context(), u.ID)
	if e != nil {
		writeError(w, 404, "contributions_not_found", "No contribution calendar is available.")
		return
	}
	writeJSON(w, 200, map[string]any{"login": u.Login, "days": days})
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var request struct {
		PlayerStart string `json:"playerStart"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil || request.PlayerStart == "" {
		writeError(w, 400, "invalid_player_harbour", "Choose a ten-week contribution harbour.")
		return
	}
	days, e := s.repo.Contributions(r.Context(), u.ID)
	if e != nil {
		writeError(w, 422, "contributions_missing", "Contribution history is unavailable.")
		return
	}
	player, e := game.FleetWindowAt(days, request.PlayerStart)
	if e != nil {
		writeError(w, 422, "invalid_player_harbour", "That ten-week contribution harbour is not playable.")
		return
	}
	enemy, e := game.SelectFleetOpponentWindow(days, player, game.SecureRand{})
	if e != nil {
		writeError(w, 422, "history_not_playable", "No different playable opponent period is available yet.")
		return
	}
	stats, e := s.repo.Stats(r.Context(), u.ID, "solo")
	if e != nil {
		writeError(w, 500, "stats_failed", "Could not load stats.")
		return
	}
	enemyDeployment, e := game.RandomDeployment(enemy.Cells, game.SecureRand{})
	if e != nil {
		writeError(w, 500, "computer_deployment_failed", "Could not prepare the computer fleet.")
		return
	}
	g := &State{ID: uuid(), Ruleset: game.ContributionFleetRuleset, Status: "deployment", Turn: "setup", PlayerBoard: player.Cells, EnemyBoard: enemy.Cells, PlayerDeployment: []game.FleetUnit{}, EnemyDeployment: enemyDeployment, FleetActions: []game.FleetAction{}, PlayerStart: player.Cells[0].Date, EnemyStart: enemy.Cells[0].Date, Stats: stats}
	if e = s.repo.CreateGame(r.Context(), u.ID, g); e != nil {
		writeError(w, 500, "game_create_failed", "Could not create the game.")
		return
	}
	writeJSON(w, 201, publicGame(g))
}

func (s *Server) deployFleet(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var request struct {
		Units []game.DeploymentChoice `json:"units"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, 400, "invalid_deployment", "A fleet deployment is required.")
		return
	}
	g, e := s.repo.DeployFleet(r.Context(), u.ID, r.PathValue("id"), request.Units)
	if e != nil {
		code := "deployment_rejected"
		if e == ErrSetupLocked {
			code = "setup_locked"
		} else if e == ErrNotFound {
			code = "game_not_found"
		}
		writeError(w, 409, code, e.Error())
		return
	}
	writeJSON(w, 200, publicGame(g))
}

func (s *Server) fleetAction(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var request struct {
		Attacker game.Coord `json:"attacker"`
		Target   game.Coord `json:"target"`
	}
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		writeError(w, 400, "invalid_action", "An attacker and target are required.")
		return
	}
	g, events, e := s.repo.ActFleet(r.Context(), u.ID, r.PathValue("id"), request.Attacker, request.Target)
	if e != nil {
		code := "action_rejected"
		switch e {
		case ErrNotFound:
			code = "game_not_found"
		case ErrGameComplete:
			code = "game_complete"
		case ErrNotYourTurn:
			code = "not_your_turn"
		}
		writeError(w, 409, code, e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"game": publicGame(g), "events": publicFleetEvents(events)})
}
func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	g, e := s.repo.Game(r.Context(), u.ID, r.PathValue("id"))
	if e != nil {
		if e == ErrLegacyGame {
			writeError(w, 410, "legacy_game_retired", "This earlier battle is historical and cannot continue under contribution_fleet_v3.")
			return
		}
		if p, ok := s.repo.(PVPRepository); ok {
			if pg, pe := p.PVPGame(r.Context(), u.ID, r.PathValue("id")); pe == nil {
				writeJSON(w, 200, pvpDTO(pg))
				return
			}
		}
		writeError(w, 404, "game_not_found", "Game was not found.")
		return
	}
	writeJSON(w, 200, publicGame(g))
}
func (s *Server) shot(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var c game.Coord
	if json.NewDecoder(r.Body).Decode(&c) != nil {
		writeError(w, 400, "invalid_coordinate", "A target coordinate is required.")
		return
	}
	g, events, e := s.repo.Shoot(r.Context(), u.ID, r.PathValue("id"), c)
	if e == ErrNotFound {
		if p, ok := s.repo.(PVPRepository); ok {
			pg, pev, pe := p.ShootPVP(r.Context(), u.ID, r.PathValue("id"), c)
			if pe == nil {
				writeJSON(w, 200, map[string]any{"game": pvpDTO(pg), "events": pev})
				return
			}
			if pe != ErrNotFound {
				writePVPError(w, pe)
				return
			}
		}
		writeError(w, 404, "game_not_found", "Game was not found.")
		return
	}
	if e != nil {
		code := "shot_rejected"
		if e == ErrGameComplete {
			code = "game_complete"
		} else if e.Error() == "duplicate shot" {
			code = "duplicate_shot"
		}
		writeError(w, 409, code, e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"game": publicGame(g), "events": events})
}
