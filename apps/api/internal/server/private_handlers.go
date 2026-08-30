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

type createReq struct {
	StartDate string      `json:"startDate"`
	Fleet     []game.Ship `json:"fleet"`
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
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
		writeError(w, 422, "contributions_missing", "Contribution history is unavailable.")
		return
	}
	idx := -1
	for i, c := range days {
		if c.Date == q.StartDate {
			idx = i / 7
			break
		}
	}
	weeks := len(days) / 7
	if idx < 0 || idx+10 > weeks {
		writeError(w, 422, "invalid_start", "Choose a complete 10-week range.")
		return
	}
	enemyIdx, e := game.OpponentStart(weeks, idx, game.SecureRand{})
	if e != nil {
		writeError(w, 422, "history_too_short", e.Error())
		return
	}
	enemyFleet, e := game.PlaceFleet(game.SecureRand{})
	if e != nil {
		writeError(w, 500, "fleet_failed", "Could not prepare enemy fleet.")
		return
	}
	stats, e := s.repo.Stats(r.Context(), u.ID, "solo")
	if e != nil {
		writeError(w, 500, "stats_failed", "Could not load stats.")
		return
	}
	g := &State{ID: uuid(), Status: "battle", Turn: "player", PlayerBoard: append([]game.Cell(nil), days[idx*7:idx*7+70]...), EnemyBoard: stripDates(days[enemyIdx*7 : enemyIdx*7+70]), PlayerFleet: q.Fleet, EnemyFleet: enemyFleet, PlayerShots: []game.Shot{}, AIShots: []game.Shot{}, PlayerStart: q.StartDate, EnemyStart: days[enemyIdx*7].Date, Stats: stats}
	if e = s.repo.CreateGame(r.Context(), u.ID, g); e != nil {
		writeError(w, 500, "game_create_failed", "Could not create the game.")
		return
	}
	writeJSON(w, 201, publicGame(g))
}
func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	g, e := s.repo.Game(r.Context(), u.ID, r.PathValue("id"))
	if e != nil {
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
		writeError(w, 409, "shot_rejected", e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"game": publicGame(g), "events": events})
}
