package server

import (
	"context"
	"sort"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func (m *MemoryRepository) CreateChallenge(_ context.Context, uid, code string, exp time.Time) (Challenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[uid]
	if !ok {
		return Challenge{}, ErrNotFound
	}
	c := &Challenge{Code: code, Status: "open", ExpiresAt: exp, Creator: u, CreatorPVP: decorate(m.stats[uid]["pvp"])}
	m.challenges[code] = c
	return *c, nil
}
func (m *MemoryRepository) PublicChallenge(_ context.Context, code string) (Challenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.challenges[code]
	if c == nil {
		return Challenge{}, ErrNotFound
	}
	if c.Status == "open" && !time.Now().Before(c.ExpiresAt) {
		c.Status = "expired"
	}
	c.CreatorPVP = decorate(m.stats[c.Creator.ID]["pvp"])
	if c.Opponent != nil {
		c.OpponentPVP = decorate(m.stats[c.Opponent.ID]["pvp"])
	}
	return *c, nil
}
func (m *MemoryRepository) AcceptChallenge(_ context.Context, uid, code string, n time.Time) (Challenge, *PVPGame, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.challenges[code]
	if c == nil {
		return Challenge{}, nil, ErrNotFound
	}
	if c.Creator.ID == uid {
		return Challenge{}, nil, ErrSelfChallenge
	}
	if c.IntendedOpponentID != "" && c.IntendedOpponentID != uid {
		return Challenge{}, nil, ErrConflict
	}
	if !n.Before(c.ExpiresAt) {
		c.Status = "expired"
		return Challenge{}, nil, ErrExpired
	}
	if c.Status != "open" {
		return Challenge{}, nil, ErrConflict
	}
	u, ok := m.users[uid]
	if !ok {
		return Challenge{}, nil, ErrNotFound
	}
	c.Opponent = &u
	c.Status = "accepted"
	c.GameID = uuid()
	g := &PVPGame{ID: c.GameID, Status: "setup", ChallengeCode: code, You: PVPPlayer{User: c.Creator, Stats: decorate(m.stats[c.Creator.ID]["pvp"])}, Opponent: PVPPlayer{User: u, Stats: decorate(m.stats[u.ID]["pvp"])}, UpdatedAt: n}
	m.pvpGames[g.ID] = g
	return *c, nil, nil
}
func (m *MemoryRepository) CancelChallenge(_ context.Context, uid, code string, n time.Time) (Challenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.challenges[code]
	if c == nil || c.Creator.ID != uid || c.Status != "open" {
		return Challenge{}, ErrConflict
	}
	if !n.Before(c.ExpiresAt) {
		c.Status = "expired"
	} else {
		c.Status = "cancelled"
	}
	return *c, nil
}
func (m *MemoryRepository) ReadyChallenge(_ context.Context, uid, code, start string, b []game.Cell, f []game.Ship, n time.Time) (Challenge, *PVPGame, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.challenges[code]
	if c == nil || c.Opponent == nil {
		return Challenge{}, nil, ErrNotFound
	}
	g := m.pvpGames[c.GameID]
	if g == nil {
		return Challenge{}, nil, ErrNotFound
	}
	pl := PVPPlayer{User: m.users[uid], Stats: decorate(m.stats[uid]["pvp"]), Board: append([]game.Cell(nil), b...), Fleet: f, Ready: true}
	if uid == c.Creator.ID {
		if c.CreatorReady {
			return Challenge{}, nil, ErrSetupLocked
		}
		c.CreatorReady = true
		g.You = pl
	} else if uid == c.Opponent.ID {
		if c.OpponentReady {
			return Challenge{}, nil, ErrSetupLocked
		}
		c.OpponentReady = true
		g.Opponent = pl
	} else {
		return Challenge{}, nil, ErrNotFound
	}
	c.Status = "ready"
	if c.CreatorReady && c.OpponentReady {
		c.Status = "battle"
		g.Status = "battle"
		g.StartingPlayer, _ = game.ChooseStartingPlayer([2]string{c.Creator.ID, c.Opponent.ID}, game.SecureRand{})
		g.CurrentTurn = g.StartingPlayer
	}
	g.UpdatedAt = n
	return *c, m.memoryPVPProjection(g, uid), nil
}
func clonePVP(g *PVPGame, uid string) *PVPGame {
	cp := *g
	if cp.You.User.ID != uid {
		cp.You, cp.Opponent = cp.Opponent, cp.You
	}
	cp.Opponent.Fleet = nil
	if cp.Status != "complete" {
		cp.Opponent.Board = stripDates(cp.Opponent.Board)
	}
	return &cp
}
func (m *MemoryRepository) PVPGame(_ context.Context, uid, id string) (*PVPGame, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.pvpGames[id]
	if g == nil || (g.You.User.ID != uid && g.Opponent.User.ID != uid) {
		return nil, ErrNotFound
	}
	return m.memoryPVPProjection(g, uid), nil
}
func (m *MemoryRepository) ShootPVP(_ context.Context, uid, id string, c game.Coord) (*PVPGame, []game.Shot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.pvpGames[id]
	if g == nil || (g.You.User.ID != uid && g.Opponent.User.ID != uid) {
		return nil, nil, ErrNotFound
	}
	if g.Status == "complete" {
		return nil, nil, ErrGameComplete
	}
	if g.CurrentTurn != uid {
		return nil, nil, ErrNotYourTurn
	}
	a, b := &g.You, &g.Opponent
	if a.User.ID != uid {
		a, b = b, a
	}
	s, n, e := game.ResolveShot(b.Fleet, a.Shots, c)
	if e != nil {
		return nil, nil, e
	}
	b.Fleet = n
	if s.Result != "sunk" {
		s.Ship = ""
	}
	a.Shots = append(a.Shots, s)
	if game.AllSunk(b.Fleet) {
		g.Status = "complete"
		g.Winner = uid
		g.CurrentTurn = ""
		g.ShareID = randomSecret(9)
		m.shares[g.ShareID] = g.ID
		finished := time.Now().UTC()
		winnerBefore := m.stats[a.User.ID]["pvp"]
		loserBefore := m.stats[b.User.ID]["pvp"]
		winnerAfter := publicStatsFromGame(game.UpdateStatsAgainst(gameStatsFromPublic(winnerBefore), loserBefore.Rating, true, len(a.Shots), countHits(a.Shots)))
		loserAfter := publicStatsFromGame(game.UpdateStatsAgainst(gameStatsFromPublic(loserBefore), winnerBefore.Rating, false, len(b.Shots), countHits(b.Shots)))
		m.stats[a.User.ID]["pvp"] = winnerAfter
		m.stats[b.User.ID]["pvp"] = loserAfter
		a.Stats, b.Stats = decorate(winnerAfter), decorate(loserAfter)
		winnerHistory := PVPHistory{GameID: g.ID, ShareID: g.ShareID, Opponent: b.User, Won: true, Shots: len(a.Shots), Hits: countHits(a.Shots), RatingDelta: winnerAfter.Rating - winnerBefore.Rating, CompletedAt: finished}
		loserHistory := PVPHistory{GameID: g.ID, ShareID: g.ShareID, Opponent: a.User, Won: false, Shots: len(b.Shots), Hits: countHits(b.Shots), RatingDelta: loserAfter.Rating - loserBefore.Rating, CompletedAt: finished}
		m.pvpHistory[a.User.ID] = append([]PVPHistory{winnerHistory}, m.pvpHistory[a.User.ID]...)
		m.pvpHistory[b.User.ID] = append([]PVPHistory{loserHistory}, m.pvpHistory[b.User.ID]...)
		m.pvpShares[g.ShareID] = PVPShare{ShareID: g.ShareID, Winner: a.User, Loser: b.User, WinnerResult: winnerHistory, LoserResult: loserHistory, Rank: decorate(winnerAfter).Rank}
		g.RatingBefore = winnerBefore.Rating
		g.RatingAfter = winnerAfter.Rating
		g.RatingDelta = winnerAfter.Rating - winnerBefore.Rating
		g.Hits = countHits(a.Shots)
		g.ShipsSunk = sunkCount(a.Shots)
	} else {
		g.CurrentTurn = b.User.ID
	}
	g.LastMove = &PVPLastMove{ShooterID: uid, Shot: s}
	g.UpdatedAt = time.Now()
	return m.memoryPVPProjection(g, uid), []game.Shot{s}, nil
}
func (m *MemoryRepository) Battles(_ context.Context, uid string) ([]BattleSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []BattleSummary{}
	for _, g := range m.pvpGames {
		if g.You.User.ID != uid && g.Opponent.User.ID != uid {
			continue
		}
		o := g.Opponent.User
		if o.ID == uid {
			o = g.You.User
		}
		out = append(out, BattleSummary{ID: g.ID, Status: g.Status, Opponent: o, YourTurn: g.CurrentTurn == uid, Winner: g.Winner, ChallengeCode: g.ChallengeCode, UpdatedAt: g.UpdatedAt})
	}
	return out, nil
}
func (m *MemoryRepository) Rematch(ctx context.Context, uid, gid string, e time.Time) (Challenge, error) {
	m.mu.Lock()
	g := m.pvpGames[gid]
	if g == nil || g.Status != "complete" || (g.You.User.ID != uid && g.Opponent.User.ID != uid) {
		m.mu.Unlock()
		return Challenge{}, ErrNotFound
	}
	if code := m.pvpRematches[gid]; code != "" {
		c := *m.challenges[code]
		m.mu.Unlock()
		return c, nil
	}
	opponentID := g.You.User.ID
	if opponentID == uid {
		opponentID = g.Opponent.User.ID
	}
	code := randomSecret(12)
	c := &Challenge{Code: code, Status: "open", ExpiresAt: e, Creator: m.users[uid], CreatorPVP: decorate(m.stats[uid]["pvp"]), IntendedOpponentID: opponentID}
	m.challenges[code] = c
	m.pvpRematches[gid] = code
	m.mu.Unlock()
	return *c, nil
}
func (m *MemoryRepository) Leaderboard(_ context.Context, limit int) ([]LeaderboardEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []LeaderboardEntry{}
	for uid, s := range m.stats {
		p := s["pvp"]
		if p.Games == 0 {
			continue
		}
		u := m.users[uid]
		out = append(out, LeaderboardEntry{Login: u.Login, Name: u.Name, AvatarURL: u.AvatarURL, Rank: decorate(p).Rank, Rating: p.Rating, Wins: p.Wins, Games: p.Games, WinRate: decorate(p).WinRate})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rating != out[j].Rating {
			return out[i].Rating > out[j].Rating
		}
		if out[i].Wins != out[j].Wins {
			return out[i].Wins > out[j].Wins
		}
		if out[i].Games != out[j].Games {
			return out[i].Games < out[j].Games
		}
		return out[i].Login < out[j].Login
	})
	if len(out) > limit {
		out = out[:limit]
	}
	for i := range out {
		out[i].Position = i + 1
	}
	return out, nil
}
func (m *MemoryRepository) PVPHistory(_ context.Context, uid string, limit int) ([]PVPHistory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := append([]PVPHistory(nil), m.pvpHistory[uid]...)
	if len(v) > limit {
		v = v[:limit]
	}
	return v, nil
}
func (m *MemoryRepository) PublicPVPShare(_ context.Context, sid string) (PVPShare, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.pvpShares[sid]
	if !ok {
		return PVPShare{}, ErrNotFound
	}
	return v, nil
}

func gameStatsFromPublic(s PublicStats) game.Stats {
	return game.Stats{Games: s.Games, Wins: s.Wins, Losses: s.Losses, Rating: s.Rating, Shots: s.Shots, Hits: s.Hits, CurrentStreak: s.CurrentStreak, LongestStreak: s.LongestStreak, WinShots: s.WinShots}
}
func publicStatsFromGame(s game.Stats) PublicStats {
	return decorate(PublicStats{Games: s.Games, Wins: s.Wins, Losses: s.Losses, Rating: s.Rating, Shots: s.Shots, Hits: s.Hits, CurrentStreak: s.CurrentStreak, LongestStreak: s.LongestStreak, WinShots: s.WinShots})
}
func sunkCount(shots []game.Shot) int {
	n := 0
	for _, s := range shots {
		if s.Result == "sunk" {
			n++
		}
	}
	return n
}
func (m *MemoryRepository) memoryPVPProjection(g *PVPGame, uid string) *PVPGame {
	cp := clonePVP(g, uid)
	if cp.Status != "complete" {
		return cp
	}
	for _, h := range m.pvpHistory[uid] {
		if h.GameID != cp.ID {
			continue
		}
		before := cp.You.Stats.Rating - h.RatingDelta
		cp.RatingBefore, cp.RatingAfter, cp.RatingDelta = before, cp.You.Stats.Rating, h.RatingDelta
		cp.Hits, cp.ShipsSunk = h.Hits, sunkCount(cp.You.Shots)
		cp.OpponentRatingAfter = cp.Opponent.Stats.Rating
		for _, oh := range m.pvpHistory[cp.Opponent.User.ID] {
			if oh.GameID == cp.ID {
				cp.OpponentRatingDelta = oh.RatingDelta
				cp.OpponentRatingBefore = cp.OpponentRatingAfter - oh.RatingDelta
				break
			}
		}
		break
	}
	return cp
}
