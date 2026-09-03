package server

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

type expiring struct {
	UserID  string
	Expires time.Time
	Used    bool
}
type MemoryRepository struct {
	mu            sync.Mutex
	users         map[string]User
	github        map[int64]string
	contributions map[string][]game.Cell
	stats         map[string]map[string]PublicStats
	oauth         map[string]expiring
	exchange      map[string]expiring
	sessions      map[string]expiring
	games         map[string]*State
	owners        map[string]string
	shares        map[string]string
	challenges    map[string]*Challenge
	pvpGames      map[string]*PVPGame
	pvpHistory    map[string][]PVPHistory
	pvpShares     map[string]PVPShare
	pvpRematches  map[string]string
	harbours      map[string]OpenHarbour
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{users: map[string]User{}, github: map[int64]string{}, contributions: map[string][]game.Cell{}, stats: map[string]map[string]PublicStats{}, oauth: map[string]expiring{}, exchange: map[string]expiring{}, sessions: map[string]expiring{}, games: map[string]*State{}, owners: map[string]string{}, shares: map[string]string{}, challenges: map[string]*Challenge{}, pvpGames: map[string]*PVPGame{}, pvpHistory: map[string][]PVPHistory{}, pvpShares: map[string]PVPShare{}, pvpRematches: map[string]string{}, harbours: map[string]OpenHarbour{}}
}
func hashKey(b []byte) string { return string(b) }
func (m *MemoryRepository) PutOAuthState(_ context.Context, h []byte, e time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.oauth[hashKey(h)] = expiring{Expires: e}
	return nil
}
func (m *MemoryRepository) ConsumeOAuthState(_ context.Context, h []byte, n time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.oauth[hashKey(h)]
	if !ok || v.Used || !n.Before(v.Expires) {
		return ErrExpired
	}
	v.Used = true
	m.oauth[hashKey(h)] = v
	return nil
}
func (m *MemoryRepository) UpsertGitHubUser(_ context.Context, u User, days []game.Cell) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	currentID := m.github[u.GitHubID]
	for candidateID, candidate := range m.users {
		if candidateID != currentID && strings.EqualFold(candidate.Login, u.Login) {
			candidate.Login += "-former-" + candidateID[:8]
			m.users[candidateID] = candidate
		}
	}
	if existingID, ok := m.github[u.GitHubID]; ok {
		existing := m.users[existingID]
		existing.Login = u.Login
		existing.Name = u.Name
		existing.AvatarURL = u.AvatarURL
		m.users[existingID] = existing
		u = existing
	} else {
		if u.ID == "" {
			u.ID = uuid()
		}
		if u.JoinedAt.IsZero() {
			u.JoinedAt = time.Now().UTC()
		}
		m.users[u.ID] = u
		m.github[u.GitHubID] = u.ID
		m.stats[u.ID] = map[string]PublicStats{"solo": decorate(PublicStats{Rating: 1200}), "solo_v2": decorate(PublicStats{Rating: 1200}), "pvp": decorate(PublicStats{Rating: 1200})}
	}
	m.contributions[u.ID] = append([]game.Cell(nil), days...)
	return u, nil
}
func (m *MemoryRepository) PutExchangeCode(_ context.Context, h []byte, uid string, e time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exchange[hashKey(h)] = expiring{UserID: uid, Expires: e}
	return nil
}
func (m *MemoryRepository) ConsumeExchangeCode(_ context.Context, h []byte, n time.Time) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := hashKey(h)
	v, ok := m.exchange[k]
	if !ok || v.Used || !n.Before(v.Expires) {
		return User{}, ErrExpired
	}
	v.Used = true
	m.exchange[k] = v
	return m.users[v.UserID], nil
}
func (m *MemoryRepository) CreateSession(_ context.Context, uid string, h []byte, e time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[hashKey(h)] = expiring{UserID: uid, Expires: e}
	return nil
}
func (m *MemoryRepository) ResolveSession(_ context.Context, h []byte, n time.Time) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.sessions[hashKey(h)]
	if !ok || v.Used || !n.Before(v.Expires) {
		return User{}, ErrUnauthorized
	}
	u, ok := m.users[v.UserID]
	if !ok {
		return User{}, ErrUnauthorized
	}
	return u, nil
}
func (m *MemoryRepository) RevokeSession(_ context.Context, h []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := hashKey(h)
	v, ok := m.sessions[k]
	if !ok {
		return ErrUnauthorized
	}
	v.Used = true
	m.sessions[k] = v
	return nil
}
func (m *MemoryRepository) Contributions(_ context.Context, uid string) ([]game.Cell, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.contributions[uid]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]game.Cell(nil), v...), nil
}
func (m *MemoryRepository) Stats(_ context.Context, uid, mode string) (PublicStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.stats[uid][mode]
	if !ok {
		return PublicStats{}, ErrNotFound
	}
	return decorate(v), nil
}
func (m *MemoryRepository) PublicUser(_ context.Context, login string) (PublicUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var u User
	found := false
	for _, candidate := range m.users {
		if strings.EqualFold(candidate.Login, login) {
			u = candidate
			found = true
			break
		}
	}
	if !found {
		return PublicUser{}, ErrNotFound
	}
	days := m.contributions[u.ID]
	sum := ContributionSummary{}
	for _, d := range days {
		sum.Total += d.ContributionCount
		if d.ContributionCount > 0 {
			sum.ActiveDays++
		}
	}
	if len(days) > 70 {
		sum.Preview = append([]game.Cell(nil), days[len(days)-70:]...)
	} else {
		sum.Preview = append([]game.Cell(nil), days...)
	}
	return PublicUser{Login: u.Login, Name: u.Name, AvatarURL: u.AvatarURL, Solo: decorate(m.stats[u.ID]["solo"]), PVP: decorate(m.stats[u.ID]["pvp"]), JoinedAt: u.JoinedAt, PublicContributionSummary: sum, PVPHistory: append([]PVPHistory(nil), m.pvpHistory[u.ID]...)}, nil
}
func (m *MemoryRepository) CreateGame(_ context.Context, uid string, g *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := cloneState(g)
	m.games[g.ID] = cp
	m.owners[g.ID] = uid
	return nil
}
func (m *MemoryRepository) Game(_ context.Context, uid, id string) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owners[id] != uid {
		return nil, ErrNotFound
	}
	g := m.games[id]
	if g == nil {
		return nil, ErrNotFound
	}
	validV4 := g.Ruleset == game.ContributionBattleshipRuleset && len(g.PlayerBoard) == game.BoardCells && len(g.EnemyBoard) == game.BoardCells
	readableV3 := g.Ruleset == game.ContributionFleetRuleset && g.Status == "complete"
	readableV2 := g.Ruleset == "contribution_targets_v2" && g.Status == "complete"
	if !validV4 && !readableV3 && !readableV2 {
		return nil, ErrLegacyGame
	}
	return cloneState(g), nil
}
func (m *MemoryRepository) DeployFleet(_ context.Context, uid, id string, choices []game.DeploymentChoice) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owners[id] != uid {
		return nil, ErrNotFound
	}
	g := m.games[id]
	if g == nil {
		return nil, ErrNotFound
	}
	if g.Ruleset != game.ContributionBattleshipRuleset {
		return nil, ErrLegacyGame
	}
	if g.Status != "deployment" || len(g.PlayerDeployment) > 0 {
		return nil, ErrSetupLocked
	}
	units, e := game.ValidateDeployment(g.PlayerBoard, choices)
	if e != nil {
		return nil, e
	}
	next := cloneState(g)
	next.PlayerDeployment = units
	next.Status = "battle"
	next.Turn = "player"
	m.games[id] = next
	return cloneState(next), nil
}
func (m *MemoryRepository) ActFleet(_ context.Context, uid, id string, attacker, target game.Coord) (*State, []game.FleetAction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owners[id] != uid {
		return nil, nil, ErrNotFound
	}
	g := m.games[id]
	if g == nil {
		return nil, nil, ErrNotFound
	}
	next := cloneState(g)
	events, stats, e := resolveFleetTurn(next, m.stats[uid]["solo"], attacker, target, game.SecureRand{})
	if e != nil {
		return nil, nil, e
	}
	m.games[id] = next
	m.stats[uid]["solo"] = stats
	if next.ShareID != "" {
		m.shares[next.ShareID] = id
	}
	return cloneState(next), events, nil
}
func (m *MemoryRepository) Shoot(_ context.Context, uid, id string, c game.Coord) (*State, []game.BattleEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owners[id] != uid {
		return nil, nil, ErrNotFound
	}
	g := m.games[id]
	if g == nil {
		return nil, nil, ErrNotFound
	}
	p := m.stats[uid]["solo_v2"]
	next := cloneState(g)
	var events []game.BattleEvent
	var np PublicStats
	var e error
	if next.Ruleset == game.ContributionBattleshipRuleset {
		p = m.stats[uid]["solo"]
		events, np, e = resolveBattleshipTurn(next, p, c, game.SecureRand{})
	} else {
		events, np, e = resolveTargetTurn(next, p, c, game.SecureRand{})
	}
	if e != nil {
		return nil, nil, e
	}
	m.games[id] = next
	if next.Ruleset == game.ContributionBattleshipRuleset {
		m.stats[uid]["solo"] = np
	} else {
		m.stats[uid]["solo_v2"] = np
	}
	if next.ShareID != "" {
		m.shares[next.ShareID] = id
	}
	return cloneState(next), events, nil
}
func (m *MemoryRepository) PublicShare(_ context.Context, sid string) (*State, User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.shares[sid]
	if !ok {
		return nil, User{}, ErrNotFound
	}
	g := m.games[id]
	if g == nil || g.Status != "complete" {
		return nil, User{}, ErrNotFound
	}
	return cloneState(g), m.users[m.owners[id]], nil
}
func (m *MemoryRepository) Close() {}
func cloneState(g *State) *State {
	if g == nil {
		return nil
	}
	var cp State
	b, _ := json.Marshal(g)
	_ = json.Unmarshal(b, &cp)
	return &cp
}
