package server

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func (m *MemoryRepository) SetOpenHarbour(_ context.Context, uid, start string, board []game.Cell, deployment []game.FleetUnit, now time.Time) (OpenHarbour, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := OpenHarbour{Owner: m.users[uid], PeriodStart: start, Board: append([]game.Cell(nil), board...), Deployment: append([]game.FleetUnit(nil), deployment...), Capacity: len(deployment), UpdatedAt: now}
	m.harbours[uid] = h
	return h, nil
}

func (m *MemoryRepository) OpenHarbour(_ context.Context, uid string) (OpenHarbour, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.harbours[uid]
	if !ok {
		return OpenHarbour{}, ErrNotFound
	}
	return h, nil
}

func (m *MemoryRepository) CloseOpenHarbour(_ context.Context, uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.harbours[uid]; !ok {
		return ErrNotFound
	}
	delete(m.harbours, uid)
	return nil
}

func (m *MemoryRepository) OpenHarbours(_ context.Context, uid string) ([]OpenHarbour, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []OpenHarbour{}
	for id, h := range m.harbours {
		if id != uid {
			h.Board = nil
			h.Deployment = nil
			out = append(out, h)
		}
	}
	return out, nil
}

func (m *MemoryRepository) StartAsyncPVP(_ context.Context, uid, opponentLogin string, now time.Time) (*AsyncPVPBattle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mine, ok := m.harbours[uid]
	if !ok {
		return nil, ErrSetupLocked
	}
	var defenderID string
	for id, u := range m.users {
		if strings.EqualFold(u.Login, opponentLogin) {
			defenderID = id
			break
		}
	}
	if defenderID == "" {
		return nil, ErrNotFound
	}
	if defenderID == uid {
		return nil, ErrSelfChallenge
	}
	theirs, ok := m.harbours[defenderID]
	if !ok {
		return nil, ErrConflict
	}
	g := &State{ID: uuid(), Ruleset: game.ContributionBattleshipRuleset, Status: "battle", Turn: "player", PlayerBoard: append([]game.Cell(nil), mine.Board...), EnemyBoard: append([]game.Cell(nil), theirs.Board...), PlayerDeployment: append([]game.FleetUnit(nil), mine.Deployment...), EnemyDeployment: append([]game.FleetUnit(nil), theirs.Deployment...), PlayerTargetShots: []game.TargetShot{}, AITargetShots: []game.TargetShot{}, PlayerStart: mine.PeriodStart, EnemyStart: theirs.PeriodStart, Stats: decorate(m.stats[uid]["pvp"]), PVPDefenderID: defenderID}
	m.games[g.ID] = g
	m.owners[g.ID] = uid
	return &AsyncPVPBattle{State: cloneState(g), Challenger: m.users[uid], Defender: m.users[defenderID], UpdatedAt: now}, nil
}

func (m *MemoryRepository) AsyncPVPGame(_ context.Context, uid, id string) (*AsyncPVPBattle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.games[id]
	if g == nil || g.PVPDefenderID == "" || (m.owners[id] != uid && g.PVPDefenderID != uid) {
		return nil, ErrNotFound
	}
	return &AsyncPVPBattle{State: cloneState(g), Challenger: m.users[m.owners[id]], Defender: m.users[g.PVPDefenderID], UpdatedAt: time.Now().UTC()}, nil
}

func (m *MemoryRepository) AsyncPVPBattles(_ context.Context, uid string) ([]*AsyncPVPBattle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*AsyncPVPBattle{}
	for id, g := range m.games {
		if g == nil || g.Ruleset != game.ContributionBattleshipRuleset || g.PVPDefenderID == "" {
			continue
		}
		challenger := m.owners[id]
		if challenger != uid && g.PVPDefenderID != uid {
			continue
		}
		out = append(out, &AsyncPVPBattle{State: cloneState(g), Challenger: m.users[challenger], Defender: m.users[g.PVPDefenderID], UpdatedAt: time.Now().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (m *MemoryRepository) ShootAsyncPVP(_ context.Context, uid, id string, c game.Coord) (*AsyncPVPBattle, []game.BattleEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.games[id]
	if g == nil || g.PVPDefenderID == "" || m.owners[id] != uid {
		return nil, nil, ErrNotFound
	}
	wasComplete := g.Status == "complete"
	events, _, err := resolveBattleshipTurn(g, m.stats[uid]["pvp"], c, game.SecureRand{})
	if err != nil {
		return nil, nil, err
	}
	if !wasComplete && g.Status == "complete" {
		m.finishAsyncPVP(g, uid, g.PVPDefenderID)
	}
	return &AsyncPVPBattle{State: cloneState(g), Challenger: m.users[uid], Defender: m.users[g.PVPDefenderID], UpdatedAt: time.Now().UTC()}, events, nil
}

func (m *MemoryRepository) finishAsyncPVP(g *State, challenger, defender string) {
	cb, db := m.stats[challenger]["pvp"], m.stats[defender]["pvp"]
	ch, _ := targetShotCounts(g.PlayerTargetShots)
	dh, _ := targetShotCounts(g.AITargetShots)
	challengerWon := g.Winner == "player"
	ca := publicStatsFromGame(game.UpdateStatsAgainst(gameStatsFromPublic(cb), db.Rating, challengerWon, len(g.PlayerTargetShots), ch))
	da := publicStatsFromGame(game.UpdateStatsAgainst(gameStatsFromPublic(db), cb.Rating, !challengerWon, len(g.AITargetShots), dh))
	m.stats[challenger]["pvp"], m.stats[defender]["pvp"] = ca, da
	g.Stats = decorate(ca)
	g.RatingDelta = ca.Rating - cb.Rating
	winner, loser := challenger, defender
	if !challengerWon {
		winner, loser = defender, challenger
	}
	finished := time.Now().UTC()
	chh := PVPHistory{GameID: g.ID, ShareID: g.ShareID, Opponent: m.users[defender], Won: challengerWon, Shots: len(g.PlayerTargetShots), Hits: ch, RatingDelta: ca.Rating - cb.Rating, CompletedAt: finished}
	dhh := PVPHistory{GameID: g.ID, ShareID: g.ShareID, Opponent: m.users[challenger], Won: !challengerWon, Shots: len(g.AITargetShots), Hits: dh, RatingDelta: da.Rating - db.Rating, CompletedAt: finished}
	m.pvpHistory[challenger] = append([]PVPHistory{chh}, m.pvpHistory[challenger]...)
	m.pvpHistory[defender] = append([]PVPHistory{dhh}, m.pvpHistory[defender]...)
	wh, lh := chh, dhh
	if winner == defender {
		wh, lh = dhh, chh
	}
	m.pvpShares[g.ShareID] = PVPShare{ShareID: g.ShareID, Winner: m.users[winner], Loser: m.users[loser], WinnerResult: wh, LoserResult: lh, Rank: decorate(m.stats[winner]["pvp"]).Rank}
}
