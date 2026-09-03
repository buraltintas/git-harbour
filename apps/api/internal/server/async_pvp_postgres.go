package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
	"github.com/jackc/pgx/v5"
)

func (p *PostgresRepository) SetOpenHarbour(ctx context.Context, uid, start string, board []game.Cell, deployment []game.FleetUnit, now time.Time) (OpenHarbour, error) {
	bb, _ := json.Marshal(board)
	db, _ := json.Marshal(deployment)
	_, e := p.pool.Exec(ctx, `INSERT INTO pvp_harbours(user_id,period_start,board_snapshot,deployment,is_open,updated_at) VALUES($1,$2,$3,$4,true,$5) ON CONFLICT(user_id) DO UPDATE SET period_start=$2,board_snapshot=$3,deployment=$4,is_open=true,updated_at=$5`, uid, start, bb, db, now)
	if e != nil {
		return OpenHarbour{}, e
	}
	return p.OpenHarbour(ctx, uid)
}
func scanOpenHarbour(row pgx.Row) (OpenHarbour, error) {
	var h OpenHarbour
	var b, d []byte
	e := row.Scan(&h.Owner.ID, &h.Owner.Login, &h.Owner.Name, &h.Owner.AvatarURL, &h.PeriodStart, &b, &d, &h.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return h, ErrNotFound
	}
	if e != nil {
		return h, e
	}
	_ = json.Unmarshal(b, &h.Board)
	_ = json.Unmarshal(d, &h.Deployment)
	h.Capacity = len(h.Deployment)
	return h, nil
}
func (p *PostgresRepository) OpenHarbour(ctx context.Context, uid string) (OpenHarbour, error) {
	return scanOpenHarbour(p.pool.QueryRow(ctx, `SELECT u.id,u.login,u.name,u.avatar_url,h.period_start::text,h.board_snapshot,h.deployment,h.updated_at FROM pvp_harbours h JOIN users u ON u.id=h.user_id WHERE h.user_id=$1 AND h.is_open`, uid))
}
func (p *PostgresRepository) CloseOpenHarbour(ctx context.Context, uid string) error {
	tag, e := p.pool.Exec(ctx, `UPDATE pvp_harbours SET is_open=false,updated_at=now() WHERE user_id=$1 AND is_open`, uid)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (p *PostgresRepository) OpenHarbours(ctx context.Context, uid string) ([]OpenHarbour, error) {
	rows, e := p.pool.Query(ctx, `SELECT u.id,u.login,u.name,u.avatar_url,h.period_start::text,h.board_snapshot,h.deployment,h.updated_at FROM pvp_harbours h JOIN users u ON u.id=h.user_id WHERE h.is_open AND h.user_id<>$1 ORDER BY h.updated_at DESC LIMIT 100`, uid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []OpenHarbour{}
	for rows.Next() {
		h, e := scanOpenHarbour(rows)
		if e != nil {
			return nil, e
		}
		h.Board = nil
		h.Deployment = nil
		out = append(out, h)
	}
	return out, rows.Err()
}

func (p *PostgresRepository) StartAsyncPVP(ctx context.Context, uid, opponentLogin string, now time.Time) (*AsyncPVPBattle, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	mine, e := scanOpenHarbour(tx.QueryRow(ctx, `SELECT u.id,u.login,u.name,u.avatar_url,h.period_start::text,h.board_snapshot,h.deployment,h.updated_at FROM pvp_harbours h JOIN users u ON u.id=h.user_id WHERE h.user_id=$1 AND h.is_open FOR SHARE`, uid))
	if e != nil {
		return nil, ErrSetupLocked
	}
	theirs, e := scanOpenHarbour(tx.QueryRow(ctx, `SELECT u.id,u.login,u.name,u.avatar_url,h.period_start::text,h.board_snapshot,h.deployment,h.updated_at FROM pvp_harbours h JOIN users u ON u.id=h.user_id WHERE lower(u.login)=lower($1) AND h.is_open FOR SHARE`, opponentLogin))
	if e != nil {
		return nil, ErrNotFound
	}
	if theirs.Owner.ID == uid {
		return nil, ErrSelfChallenge
	}
	stats, e := scanStats(tx.QueryRow(ctx, `SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM ruleset_mode_stats WHERE user_id=$1 AND mode='pvp' AND ruleset=$2`, uid, game.ContributionBattleshipRuleset))
	if e != nil {
		return nil, e
	}
	g := &State{ID: uuid(), Ruleset: game.ContributionBattleshipRuleset, Status: "battle", Turn: "player", PlayerBoard: mine.Board, EnemyBoard: theirs.Board, PlayerDeployment: mine.Deployment, EnemyDeployment: theirs.Deployment, PlayerTargetShots: []game.TargetShot{}, AITargetShots: []game.TargetShot{}, PlayerStart: mine.PeriodStart, EnemyStart: theirs.PeriodStart, Stats: stats, PVPDefenderID: theirs.Owner.ID}
	b, _ := json.Marshal(g)
	_, e = tx.Exec(ctx, `INSERT INTO games(id,mode,ruleset,status,current_turn,player_id,player_start,enemy_start,state,starting_player_id,current_turn_user_id,terminal_applied,updated_at) VALUES($1,'pvp',$2,'battle','player',$3,$4,$5,$6,$3,$3,false,$7)`, g.ID, g.Ruleset, uid, g.PlayerStart, g.EnemyStart, b, now)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return &AsyncPVPBattle{State: g, Challenger: mine.Owner, Defender: theirs.Owner, UpdatedAt: now}, nil
}

func (p *PostgresRepository) AsyncPVPGame(ctx context.Context, uid, id string) (*AsyncPVPBattle, error) {
	var b []byte
	var terminal bool
	var a AsyncPVPBattle
	e := p.pool.QueryRow(ctx, `SELECT g.state,g.terminal_applied,g.updated_at,cu.id,cu.login,cu.name,cu.avatar_url,du.id,du.login,du.name,du.avatar_url FROM games g JOIN users cu ON cu.id=g.player_id JOIN users du ON du.id=(g.state->>'pvpDefenderId')::uuid WHERE g.id=$1 AND g.mode='pvp' AND g.ruleset=$2 AND $3 IN (cu.id,du.id)`, id, game.ContributionBattleshipRuleset, uid).Scan(&b, &terminal, &a.UpdatedAt, &a.Challenger.ID, &a.Challenger.Login, &a.Challenger.Name, &a.Challenger.AvatarURL, &a.Defender.ID, &a.Defender.Login, &a.Defender.Name, &a.Defender.AvatarURL)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	var g State
	if e = json.Unmarshal(b, &g); e != nil {
		return nil, e
	}
	g.TerminalApplied = terminal
	a.State = &g
	return &a, nil
}

func (p *PostgresRepository) AsyncPVPBattles(ctx context.Context, uid string) ([]*AsyncPVPBattle, error) {
	rows, e := p.pool.Query(ctx, `SELECT g.state,g.terminal_applied,g.updated_at,cu.id,cu.login,cu.name,cu.avatar_url,du.id,du.login,du.name,du.avatar_url FROM games g JOIN users cu ON cu.id=g.player_id JOIN users du ON du.id=(g.state->>'pvpDefenderId')::uuid WHERE g.mode='pvp' AND g.ruleset=$1 AND $2 IN (cu.id,du.id) ORDER BY g.updated_at DESC LIMIT 100`, game.ContributionBattleshipRuleset, uid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []*AsyncPVPBattle{}
	for rows.Next() {
		var b []byte
		var terminal bool
		var a AsyncPVPBattle
		if e = rows.Scan(&b, &terminal, &a.UpdatedAt, &a.Challenger.ID, &a.Challenger.Login, &a.Challenger.Name, &a.Challenger.AvatarURL, &a.Defender.ID, &a.Defender.Login, &a.Defender.Name, &a.Defender.AvatarURL); e != nil {
			return nil, e
		}
		var g State
		if e = json.Unmarshal(b, &g); e != nil {
			return nil, e
		}
		g.TerminalApplied = terminal
		a.State = &g
		out = append(out, &a)
	}
	return out, rows.Err()
}

func (p *PostgresRepository) ShootAsyncPVP(ctx context.Context, uid, id string, c game.Coord) (*AsyncPVPBattle, []game.BattleEvent, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return nil, nil, e
	}
	defer tx.Rollback(ctx)
	var b []byte
	var defender string
	var terminal bool
	e = tx.QueryRow(ctx, `SELECT state,state->>'pvpDefenderId',terminal_applied FROM games WHERE id=$1 AND mode='pvp' AND ruleset=$2 AND player_id=$3 FOR UPDATE`, id, game.ContributionBattleshipRuleset, uid).Scan(&b, &defender, &terminal)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if e != nil {
		return nil, nil, e
	}
	var g State
	if e = json.Unmarshal(b, &g); e != nil {
		return nil, nil, e
	}
	g.TerminalApplied = terminal
	cs, e := scanStats(tx.QueryRow(ctx, `SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM ruleset_mode_stats WHERE user_id=$1 AND mode='pvp' AND ruleset=$2 FOR UPDATE`, uid, g.Ruleset))
	if e != nil {
		return nil, nil, e
	}
	ds, e := scanStats(tx.QueryRow(ctx, `SELECT games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM ruleset_mode_stats WHERE user_id=$1 AND mode='pvp' AND ruleset=$2 FOR UPDATE`, defender, g.Ruleset))
	if e != nil {
		return nil, nil, e
	}
	events, _, e := resolveBattleshipTurn(&g, cs, c, game.SecureRand{})
	if e != nil {
		return nil, nil, e
	}
	if g.Status == "complete" && !terminal {
		if e = p.finishAsyncPVP(ctx, tx, &g, uid, defender, cs, ds); e != nil {
			return nil, nil, e
		}
	}
	b, _ = json.Marshal(&g)
	_, e = tx.Exec(ctx, `UPDATE games SET state=$2,status=$3,current_turn=$4,winner=$5,terminal_applied=$6,current_turn_user_id=CASE WHEN $3='complete' THEN NULL ELSE player_id END,winner_user_id=CASE WHEN $3='complete' THEN CASE WHEN $5='player' THEN player_id ELSE ($2::jsonb->>'pvpDefenderId')::uuid END END,completed_at=CASE WHEN $3='complete' THEN now() END,updated_at=now() WHERE id=$1`, id, b, g.Status, g.Turn, nilIfEmpty(g.Winner), g.TerminalApplied)
	if e != nil {
		return nil, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, nil, e
	}
	a, e := p.AsyncPVPGame(ctx, uid, id)
	return a, events, e
}

func (p *PostgresRepository) finishAsyncPVP(ctx context.Context, tx pgx.Tx, g *State, challenger, defender string, cs, ds PublicStats) error {
	ch, _ := targetShotCounts(g.PlayerTargetShots)
	dh, _ := targetShotCounts(g.AITargetShots)
	cw := g.Winner == "player"
	ca := publicStatsFromGame(game.UpdateStatsAgainst(gameStatsFromPublic(cs), ds.Rating, cw, len(g.PlayerTargetShots), ch))
	da := publicStatsFromGame(game.UpdateStatsAgainst(gameStatsFromPublic(ds), cs.Rating, !cw, len(g.AITargetShots), dh))
	g.Stats = ca
	g.RatingDelta = ca.Rating - cs.Rating
	for _, x := range []struct {
		uid, opp             string
		before, after        PublicStats
		won                  bool
		shots, hits, targets int
	}{{challenger, defender, cs, ca, cw, len(g.PlayerTargetShots), ch, len(g.EnemyDeployment)}, {defender, challenger, ds, da, !cw, len(g.AITargetShots), dh, len(g.PlayerDeployment)}} {
		_, e := tx.Exec(ctx, `UPDATE ruleset_mode_stats SET games=$3,wins=$4,losses=$5,rating=$6,shots=$7,hits=$8,current_streak=$9,longest_streak=$10,win_shots=$11 WHERE user_id=$1 AND mode='pvp' AND ruleset=$2`, x.uid, g.Ruleset, x.after.Games, x.after.Wins, x.after.Losses, x.after.Rating, x.after.Shots, x.after.Hits, x.after.CurrentStreak, x.after.LongestStreak, x.after.WinShots)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO pvp_results(game_id,user_id,opponent_id,won,shots,hits,ships_sunk,rating_before,rating_after,rating_delta,target_count,targets_hit,misses) VALUES($1,$2,$3,$4,$5,$6,$6,$7,$8,$9,$10,$6,$11)`, g.ID, x.uid, x.opp, x.won, x.shots, x.hits, x.before.Rating, x.after.Rating, x.after.Rating-x.before.Rating, x.targets, x.shots-x.hits)
		if e != nil {
			return e
		}
	}
	_, e := tx.Exec(ctx, `INSERT INTO shares(id,game_id) VALUES($1,$2)`, g.ShareID, g.ID)
	return e
}
