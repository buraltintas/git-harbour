package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}
func (p *PostgresRepository) Close() { p.pool.Close() }
func (p *PostgresRepository) PutOAuthState(ctx context.Context, h []byte, e time.Time) error {
	_, x := p.pool.Exec(ctx, `INSERT INTO oauth_states(state_hash,expires_at) VALUES($1,$2)`, h, e)
	return x
}
func (p *PostgresRepository) ConsumeOAuthState(ctx context.Context, h []byte, n time.Time) error {
	tag, e := p.pool.Exec(ctx, `UPDATE oauth_states SET consumed_at=$2 WHERE state_hash=$1 AND consumed_at IS NULL AND expires_at>$2`, h, n)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrExpired
	}
	return nil
}
func (p *PostgresRepository) UpsertGitHubUser(ctx context.Context, u User, days []game.Cell) (User, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return User{}, e
	}
	defer tx.Rollback(ctx)
	var id string
	var joined time.Time
	e = tx.QueryRow(ctx, `SELECT user_id FROM github_identities WHERE github_id=$1 FOR UPDATE`, u.GitHubID).Scan(&id)
	isNew := errors.Is(e, pgx.ErrNoRows)
	if isNew {
		id = uuid()
	} else if e != nil {
		return User{}, e
	}
	// GitHub IDs are durable; release a stale login that GitHub has reassigned.
	if _, e = tx.Exec(ctx, `UPDATE users SET login=login||'-former-'||left(id::text,8),updated_at=now() WHERE lower(login)=lower($1) AND id<>$2`, u.Login, id); e != nil {
		return User{}, e
	}
	if isNew {
		e = tx.QueryRow(ctx, `INSERT INTO users(id,login,name,avatar_url) VALUES($1,$2,$3,$4) RETURNING created_at`, id, u.Login, u.Name, u.AvatarURL).Scan(&joined)
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO github_identities(user_id,github_id) VALUES($1,$2)`, id, u.GitHubID)
		}
	} else {
		e = tx.QueryRow(ctx, `UPDATE users SET login=$2,name=$3,avatar_url=$4,updated_at=now() WHERE id=$1 RETURNING created_at`, id, u.Login, u.Name, u.AvatarURL).Scan(&joined)
	}
	if e != nil {
		return User{}, e
	}
	if _, e = tx.Exec(ctx, `DELETE FROM contribution_days WHERE user_id=$1`, id); e != nil {
		return User{}, e
	}
	rows := make([][]any, 0, len(days))
	for _, d := range days {
		rows = append(rows, []any{id, d.Date, d.ContributionCount, d.ContributionLevel})
	}
	if len(rows) > 0 {
		if _, e = tx.CopyFrom(ctx, pgx.Identifier{"contribution_days"}, []string{"user_id", "day", "contribution_count", "contribution_level"}, pgx.CopyFromRows(rows)); e != nil {
			return User{}, e
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO mode_stats(user_id,mode) VALUES($1,'solo'),($1,'pvp') ON CONFLICT DO NOTHING`, id); e != nil {
		return User{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO ruleset_mode_stats(user_id,mode,ruleset) VALUES($1,'solo','contribution_targets_v2'),($1,'solo','contribution_fleet_v3') ON CONFLICT DO NOTHING`, id); e != nil {
		return User{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return User{}, e
	}
	u.ID = id
	u.JoinedAt = joined
	return u, nil
}
func (p *PostgresRepository) PutExchangeCode(ctx context.Context, h []byte, uid string, e time.Time) error {
	_, x := p.pool.Exec(ctx, `INSERT INTO login_exchange_codes(code_hash,user_id,expires_at) VALUES($1,$2,$3)`, h, uid, e)
	return x
}
func (p *PostgresRepository) ConsumeExchangeCode(ctx context.Context, h []byte, n time.Time) (User, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return User{}, e
	}
	defer tx.Rollback(ctx)
	var uid string
	e = tx.QueryRow(ctx, `UPDATE login_exchange_codes SET consumed_at=$2 WHERE code_hash=$1 AND consumed_at IS NULL AND expires_at>$2 RETURNING user_id`, h, n).Scan(&uid)
	if errors.Is(e, pgx.ErrNoRows) {
		return User{}, ErrExpired
	}
	if e != nil {
		return User{}, e
	}
	u, e := userByID(ctx, tx, uid)
	if e != nil {
		return User{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return User{}, e
	}
	return u, nil
}
func (p *PostgresRepository) CreateSession(ctx context.Context, uid string, h []byte, e time.Time) error {
	_, x := p.pool.Exec(ctx, `INSERT INTO auth_sessions(id,user_id,token_hash,expires_at) VALUES($1,$2,$3,$4)`, uuid(), uid, h, e)
	return x
}
func (p *PostgresRepository) ResolveSession(ctx context.Context, h []byte, n time.Time) (User, error) {
	var u User
	e := p.pool.QueryRow(ctx, `UPDATE auth_sessions s SET last_seen_at=$2 FROM users u JOIN github_identities gi ON gi.user_id=u.id WHERE s.token_hash=$1 AND s.user_id=u.id AND s.revoked_at IS NULL AND s.expires_at>$2 RETURNING u.id,u.login,u.name,u.avatar_url,gi.github_id,u.created_at`, h, n).Scan(&u.ID, &u.Login, &u.Name, &u.AvatarURL, &u.GitHubID, &u.JoinedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	return u, e
}
func (p *PostgresRepository) RevokeSession(ctx context.Context, h []byte) error {
	tag, e := p.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, h)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrUnauthorized
	}
	return nil
}
func (p *PostgresRepository) Contributions(ctx context.Context, uid string) ([]game.Cell, error) {
	rows, e := p.pool.Query(ctx, `SELECT day::text,extract(dow from day)::int,contribution_count,contribution_level FROM contribution_days WHERE user_id=$1 ORDER BY day`, uid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []game.Cell{}
	for rows.Next() {
		var d game.Cell
		if e = rows.Scan(&d.Date, &d.Weekday, &d.ContributionCount, &d.ContributionLevel); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out, rows.Err()
}
func scanStats(row pgx.Row) (PublicStats, error) {
	var s PublicStats
	e := row.Scan(&s.Games, &s.Wins, &s.Losses, &s.Rating, &s.Shots, &s.Hits, &s.CurrentStreak, &s.LongestStreak, &s.WinShots)
	if errors.Is(e, pgx.ErrNoRows) {
		return PublicStats{}, ErrNotFound
	}
	return decorate(s), e
}
func (p *PostgresRepository) Stats(ctx context.Context, uid, mode string) (PublicStats, error) {
	if mode == "solo" {
		return scanStats(p.pool.QueryRow(ctx, `SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM ruleset_mode_stats WHERE user_id=$1 AND mode='solo' AND ruleset='contribution_fleet_v3'`, uid))
	}
	return scanStats(p.pool.QueryRow(ctx, `SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM mode_stats WHERE user_id=$1 AND mode=$2`, uid, mode))
}
func (p *PostgresRepository) PublicUser(ctx context.Context, login string) (PublicUser, error) {
	var u PublicUser
	var uid string
	e := p.pool.QueryRow(ctx, `SELECT id,login,name,avatar_url,created_at FROM users WHERE lower(login)=lower($1)`, login).Scan(&uid, &u.Login, &u.Name, &u.AvatarURL, &u.JoinedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return PublicUser{}, ErrNotFound
	}
	if e != nil {
		return u, e
	}
	u.Solo, e = p.Stats(ctx, uid, "solo")
	if e != nil {
		return u, e
	}
	u.PVP, e = p.Stats(ctx, uid, "pvp")
	if e != nil {
		return u, e
	}
	rows, e := p.pool.Query(ctx, `SELECT day::text,extract(dow from day)::int,contribution_count,contribution_level FROM contribution_days WHERE user_id=$1 ORDER BY day DESC LIMIT 70`, uid)
	if e != nil {
		return u, e
	}
	defer rows.Close()
	preview := []game.Cell{}
	for rows.Next() {
		var d game.Cell
		if e = rows.Scan(&d.Date, &d.Weekday, &d.ContributionCount, &d.ContributionLevel); e != nil {
			return u, e
		}
		preview = append(preview, d)
		u.PublicContributionSummary.Total += d.ContributionCount
		if d.ContributionCount > 0 {
			u.PublicContributionSummary.ActiveDays++
		}
	}
	for i, j := 0, len(preview)-1; i < j; i, j = i+1, j-1 {
		preview[i], preview[j] = preview[j], preview[i]
	}
	u.PublicContributionSummary.Preview = preview
	u.PVPHistory, e = p.PVPHistory(ctx, uid, 10)
	if e != nil {
		return u, e
	}
	return u, rows.Err()
}
func (p *PostgresRepository) CreateGame(ctx context.Context, uid string, g *State) error {
	b, _ := json.Marshal(g)
	_, e := p.pool.Exec(ctx, `INSERT INTO games(id,mode,ruleset,status,current_turn,winner,player_id,player_start,enemy_start,state,terminal_applied) VALUES($1,'solo',$2,$3,$4,NULL,$5,$6,$7,$8,false)`, g.ID, g.Ruleset, g.Status, g.Turn, uid, g.PlayerStart, g.EnemyStart, b)
	return e
}
func (p *PostgresRepository) Game(ctx context.Context, uid, id string) (*State, error) {
	var b []byte
	var ruleset string
	e := p.pool.QueryRow(ctx, `SELECT ruleset,state FROM games WHERE id=$1 AND player_id=$2 AND mode='solo'`, id, uid).Scan(&ruleset, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	var g State
	e = json.Unmarshal(b, &g)
	g.Ruleset = ruleset
	validV3 := ruleset == game.ContributionFleetRuleset && len(g.PlayerBoard) == game.BoardCells && len(g.EnemyBoard) == game.BoardCells
	readableV2 := ruleset == "contribution_targets_v2" && g.Status == "complete"
	if e == nil && !validV3 && !readableV2 {
		return nil, ErrLegacyGame
	}
	return &g, e
}

func (p *PostgresRepository) DeployFleet(ctx context.Context, uid, id string, choices []game.DeploymentChoice) (*State, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	var b []byte
	var ruleset string
	e = tx.QueryRow(ctx, `SELECT ruleset,state FROM games WHERE id=$1 AND player_id=$2 AND mode='solo' FOR UPDATE`, id, uid).Scan(&ruleset, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	if ruleset != game.ContributionFleetRuleset {
		return nil, ErrLegacyGame
	}
	var g State
	if e = json.Unmarshal(b, &g); e != nil {
		return nil, e
	}
	if g.Status != "deployment" || len(g.PlayerDeployment) > 0 {
		return nil, ErrSetupLocked
	}
	units, e := game.ValidateDeployment(g.PlayerBoard, choices)
	if e != nil {
		return nil, e
	}
	g.PlayerDeployment = units
	g.Status = "battle"
	g.Turn = "player"
	b, _ = json.Marshal(&g)
	if _, e = tx.Exec(ctx, `UPDATE games SET status=$2,current_turn=$3,state=$4,updated_at=now() WHERE id=$1`, id, g.Status, g.Turn, b); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return &g, nil
}

func (p *PostgresRepository) ActFleet(ctx context.Context, uid, id string, attacker, target game.Coord) (*State, []game.FleetAction, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return nil, nil, e
	}
	defer tx.Rollback(ctx)
	var b []byte
	var ruleset string
	e = tx.QueryRow(ctx, `SELECT ruleset,state FROM games WHERE id=$1 AND player_id=$2 AND mode='solo' FOR UPDATE`, id, uid).Scan(&ruleset, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if e != nil {
		return nil, nil, e
	}
	if ruleset != game.ContributionFleetRuleset {
		return nil, nil, ErrLegacyGame
	}
	var g State
	if e = json.Unmarshal(b, &g); e != nil {
		return nil, nil, e
	}
	g.Ruleset = ruleset
	s, e := scanStats(tx.QueryRow(ctx, `SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM ruleset_mode_stats WHERE user_id=$1 AND mode='solo' AND ruleset='contribution_fleet_v3' FOR UPDATE`, uid))
	if e != nil {
		return nil, nil, e
	}
	events, updated, e := resolveFleetTurn(&g, s, attacker, target, game.SecureRand{})
	if e != nil {
		return nil, nil, e
	}
	b, _ = json.Marshal(&g)
	if _, e = tx.Exec(ctx, `UPDATE games SET status=$2,current_turn=$3,winner=$4,state=$5,terminal_applied=$6,updated_at=now() WHERE id=$1`, id, g.Status, g.Turn, nilIfEmpty(g.Winner), b, g.TerminalApplied); e != nil {
		return nil, nil, e
	}
	if g.TerminalApplied {
		if _, e = tx.Exec(ctx, `UPDATE ruleset_mode_stats SET games=$2,wins=$3,losses=$4,rating=$5,shots=$6,hits=$7,current_streak=$8,longest_streak=$9,win_shots=$10 WHERE user_id=$1 AND mode='solo' AND ruleset='contribution_fleet_v3'`, uid, updated.Games, updated.Wins, updated.Losses, updated.Rating, updated.Shots, updated.Hits, updated.CurrentStreak, updated.LongestStreak, updated.WinShots); e != nil {
			return nil, nil, e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO shares(id,game_id) VALUES($1,$2) ON CONFLICT(game_id) DO NOTHING`, g.ShareID, id); e != nil {
			return nil, nil, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, nil, e
	}
	return &g, events, nil
}
func (p *PostgresRepository) Shoot(ctx context.Context, uid, id string, c game.Coord) (*State, []game.BattleEvent, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return nil, nil, e
	}
	defer tx.Rollback(ctx)
	var b []byte
	var ruleset string
	e = tx.QueryRow(ctx, `SELECT ruleset,state FROM games WHERE id=$1 AND player_id=$2 AND mode='solo' FOR UPDATE`, id, uid).Scan(&ruleset, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if e != nil {
		return nil, nil, e
	}
	if ruleset != "contribution_targets_v2" {
		return nil, nil, ErrLegacyGame
	}
	var g State
	if e = json.Unmarshal(b, &g); e != nil {
		return nil, nil, e
	}
	g.Ruleset = ruleset
	s, e := scanStats(tx.QueryRow(ctx, `SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM ruleset_mode_stats WHERE user_id=$1 AND mode='solo' AND ruleset='contribution_targets_v2' FOR UPDATE`, uid))
	if e != nil {
		return nil, nil, e
	}
	events, updated, e := resolveTargetTurn(&g, s, c, game.SecureRand{})
	if e != nil {
		return nil, nil, e
	}
	b, _ = json.Marshal(&g)
	if _, e = tx.Exec(ctx, `UPDATE games SET status=$2,current_turn=$3,winner=$4,state=$5,terminal_applied=$6,updated_at=now() WHERE id=$1`, id, g.Status, g.Turn, nilIfEmpty(g.Winner), b, g.TerminalApplied); e != nil {
		return nil, nil, e
	}
	if g.TerminalApplied {
		if _, e = tx.Exec(ctx, `UPDATE ruleset_mode_stats SET games=$2,wins=$3,losses=$4,rating=$5,shots=$6,hits=$7,current_streak=$8,longest_streak=$9,win_shots=$10 WHERE user_id=$1 AND mode='solo' AND ruleset='contribution_targets_v2'`, uid, updated.Games, updated.Wins, updated.Losses, updated.Rating, updated.Shots, updated.Hits, updated.CurrentStreak, updated.LongestStreak, updated.WinShots); e != nil {
			return nil, nil, e
		}
		if _, e = tx.Exec(ctx, `INSERT INTO shares(id,game_id) VALUES($1,$2) ON CONFLICT(game_id) DO NOTHING`, g.ShareID, id); e != nil {
			return nil, nil, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, nil, e
	}
	return &g, events, nil
}
func (p *PostgresRepository) PublicShare(ctx context.Context, sid string) (*State, User, error) {
	var b []byte
	var u User
	e := p.pool.QueryRow(ctx, `SELECT g.state,u.id,u.login,u.name,u.avatar_url,gi.github_id,u.created_at FROM shares s JOIN games g ON g.id=s.game_id JOIN users u ON u.id=g.player_id JOIN github_identities gi ON gi.user_id=u.id WHERE s.id=$1 AND g.status='complete'`, sid).Scan(&b, &u.ID, &u.Login, &u.Name, &u.AvatarURL, &u.GitHubID, &u.JoinedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, User{}, ErrNotFound
	}
	if e != nil {
		return nil, User{}, e
	}
	var g State
	e = json.Unmarshal(b, &g)
	return &g, u, e
}
func userByID(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (User, error) {
	var u User
	e := q.QueryRow(ctx, `SELECT u.id,u.login,u.name,u.avatar_url,gi.github_id,u.created_at FROM users u JOIN github_identities gi ON gi.user_id=u.id WHERE u.id=$1`, id).Scan(&u.ID, &u.Login, &u.Name, &u.AvatarURL, &u.GitHubID, &u.JoinedAt)
	return u, e
}
func nilIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
