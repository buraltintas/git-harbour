package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
	"github.com/jackc/pgx/v5"
)

func challengeScan(row pgx.Row) (Challenge, error) {
	var c Challenge
	var oid, gid, intended *string
	var ou User
	var on, oa *string
	e := row.Scan(&c.Code, &c.Status, &c.ExpiresAt, &c.Creator.ID, &c.Creator.Login, &c.Creator.Name, &c.Creator.AvatarURL, &oid, &on, &oa, &gid, &intended, &c.CreatorReady, &c.OpponentReady)
	if errors.Is(e, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	if e != nil {
		return c, e
	}
	if oid != nil {
		ou.ID = *oid
		if on != nil {
			ou.Login = *on
		}
		if oa != nil {
			ou.AvatarURL = *oa
		}
		c.Opponent = &ou
	}
	if gid != nil {
		c.GameID = *gid
	}
	if intended != nil {
		c.IntendedOpponentID = *intended
	}
	return c, nil
}

const challengeSQL = `SELECT c.code,c.status,c.expires_at,cu.id,cu.login,cu.name,cu.avatar_url,ou.id,ou.login,ou.avatar_url,c.game_id,c.intended_opponent_id,
 EXISTS(SELECT 1 FROM game_players gp WHERE gp.game_id=c.game_id AND gp.user_id=c.creator_id),
 EXISTS(SELECT 1 FROM game_players gp WHERE gp.game_id=c.game_id AND gp.user_id=c.opponent_id)
 FROM challenges c JOIN users cu ON cu.id=c.creator_id LEFT JOIN users ou ON ou.id=c.opponent_id WHERE c.code=$1`

func (p *PostgresRepository) CreateChallenge(ctx context.Context, uid, code string, expires time.Time) (Challenge, error) {
	_, e := p.pool.Exec(ctx, `INSERT INTO challenges(id,code,creator_id,status,expires_at) VALUES($1,$2,$3,'open',$4)`, uuid(), code, uid, expires)
	if e != nil {
		return Challenge{}, e
	}
	return p.PublicChallenge(ctx, code)
}
func (p *PostgresRepository) PublicChallenge(ctx context.Context, code string) (Challenge, error) {
	c, e := challengeScan(p.pool.QueryRow(ctx, challengeSQL, code))
	if e == nil {
		c.CreatorPVP, _ = p.Stats(ctx, c.Creator.ID, "pvp")
		if c.Opponent != nil {
			c.OpponentPVP, _ = p.Stats(ctx, c.Opponent.ID, "pvp")
		}
	}
	if e == nil && c.Status == "open" && !time.Now().UTC().Before(c.ExpiresAt) {
		_, _ = p.pool.Exec(ctx, `UPDATE challenges SET status='expired' WHERE code=$1 AND status='open'`, code)
		c.Status = "expired"
	}
	return c, e
}
func (p *PostgresRepository) AcceptChallenge(ctx context.Context, uid, code string, now time.Time) (Challenge, *PVPGame, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return Challenge{}, nil, e
	}
	defer tx.Rollback(ctx)
	var creator, status string
	var expires time.Time
	var intended *string
	e = tx.QueryRow(ctx, `SELECT creator_id,status,expires_at,intended_opponent_id FROM challenges WHERE code=$1 FOR UPDATE`, code).Scan(&creator, &status, &expires, &intended)
	if errors.Is(e, pgx.ErrNoRows) {
		return Challenge{}, nil, ErrNotFound
	}
	if e != nil {
		return Challenge{}, nil, e
	}
	if creator == uid {
		return Challenge{}, nil, ErrSelfChallenge
	}
	if status != "open" {
		return Challenge{}, nil, ErrConflict
	}
	if !now.Before(expires) {
		_, e = tx.Exec(ctx, `UPDATE challenges SET status='expired' WHERE code=$1`, code)
		if e == nil {
			e = tx.Commit(ctx)
		}
		return Challenge{}, nil, ErrExpired
	}
	if intended != nil && *intended != uid {
		return Challenge{}, nil, ErrConflict
	}
	gid := uuid()
	_, e = tx.Exec(ctx, `INSERT INTO games(id,mode,status,current_turn,player_id,state) VALUES($1,'pvp','setup','pending',$2,'{}')`, gid, creator)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE challenges SET opponent_id=$2,game_id=$3,status='accepted' WHERE code=$1`, code, uid, gid)
	}
	if e != nil {
		return Challenge{}, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Challenge{}, nil, e
	}
	c, e := p.PublicChallenge(ctx, code)
	return c, nil, e
}
func (p *PostgresRepository) CancelChallenge(ctx context.Context, uid, code string, now time.Time) (Challenge, error) {
	tag, e := p.pool.Exec(ctx, `UPDATE challenges SET status=CASE WHEN expires_at<=$3 THEN 'expired' ELSE 'cancelled' END WHERE code=$1 AND creator_id=$2 AND status='open'`, code, uid, now)
	if e != nil {
		return Challenge{}, e
	}
	if tag.RowsAffected() != 1 {
		return Challenge{}, ErrConflict
	}
	return p.PublicChallenge(ctx, code)
}
func (p *PostgresRepository) ReadyChallenge(ctx context.Context, uid, code, start string, board []game.Cell, fleet []game.Ship, now time.Time) (Challenge, *PVPGame, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return Challenge{}, nil, e
	}
	defer tx.Rollback(ctx)
	var gid, creator, opponent, status string
	e = tx.QueryRow(ctx, `SELECT game_id,creator_id,opponent_id,status FROM challenges WHERE code=$1 FOR UPDATE`, code).Scan(&gid, &creator, &opponent, &status)
	if errors.Is(e, pgx.ErrNoRows) {
		return Challenge{}, nil, ErrNotFound
	}
	if e != nil {
		return Challenge{}, nil, e
	}
	if uid != creator && uid != opponent {
		return Challenge{}, nil, ErrNotFound
	}
	if status != "accepted" && status != "ready" {
		return Challenge{}, nil, ErrConflict
	}
	side := "opponent"
	if uid == creator {
		side = "player"
	}
	bb, _ := json.Marshal(board)
	fb, _ := json.Marshal(fleet)
	_, e = tx.Exec(ctx, `INSERT INTO game_players(game_id,user_id,side,board_snapshot,fleet,shots,period_start,ready_at) VALUES($1,$2,$3,$4,$5,'[]',$6,$7)`, gid, uid, side, bb, fb, start, now)
	if e != nil {
		if isUnique(e) {
			return Challenge{}, nil, ErrSetupLocked
		}
		return Challenge{}, nil, e
	}
	var n int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM game_players WHERE game_id=$1`, gid).Scan(&n)
	if n == 2 {
		pick, re := game.ChooseStartingPlayer([2]string{creator, opponent}, game.SecureRand{})
		if re != nil {
			return Challenge{}, nil, re
		}
		_, e = tx.Exec(ctx, `UPDATE games SET status='battle',starting_player_id=$2,current_turn_user_id=$2,current_turn='player',updated_at=$3 WHERE id=$1`, gid, pick, now)
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE challenges SET status='battle' WHERE code=$1`, code)
		}
	} else {
		_, e = tx.Exec(ctx, `UPDATE challenges SET status='ready' WHERE code=$1`, code)
	}
	if e != nil {
		return Challenge{}, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Challenge{}, nil, e
	}
	c, e := p.PublicChallenge(ctx, code)
	if e != nil {
		return c, nil, e
	}
	g, _ := p.PVPGame(ctx, uid, gid)
	return c, g, nil
}
func isUnique(e error) bool {
	var pe interface{ SQLState() string }
	return errors.As(e, &pe) && pe.SQLState() == "23505"
}

type pvpRow struct {
	uid   string
	user  User
	board []game.Cell
	fleet []game.Ship
}

func (p *PostgresRepository) PVPGame(ctx context.Context, uid, id string) (*PVPGame, error) {
	var status string
	var turn, winner, start *string
	var updated time.Time
	var code, share string
	e := p.pool.QueryRow(ctx, `SELECT g.status,g.current_turn_user_id,g.winner_user_id,g.starting_player_id,g.updated_at,c.code,coalesce(s.id,'') FROM games g JOIN game_players mine ON mine.game_id=g.id AND mine.user_id=$2 JOIN challenges c ON c.game_id=g.id LEFT JOIN shares s ON s.game_id=g.id WHERE g.id=$1 AND g.mode='pvp'`, id, uid).Scan(&status, &turn, &winner, &start, &updated, &code, &share)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	rows, e := p.pool.Query(ctx, `SELECT gp.user_id,u.login,u.name,u.avatar_url,gp.board_snapshot,gp.fleet FROM game_players gp JOIN users u ON u.id=gp.user_id WHERE gp.game_id=$1`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var ps []pvpRow
	for rows.Next() {
		var x pvpRow
		var b, f []byte
		if e = rows.Scan(&x.uid, &x.user.Login, &x.user.Name, &x.user.AvatarURL, &b, &f); e != nil {
			return nil, e
		}
		x.user.ID = x.uid
		_ = json.Unmarshal(b, &x.board)
		_ = json.Unmarshal(f, &x.fleet)
		ps = append(ps, x)
	}
	if len(ps) != 2 {
		return nil, ErrConflict
	}
	g := &PVPGame{ID: id, Status: status, UpdatedAt: updated, ChallengeCode: code, ShareID: share}
	if turn != nil {
		g.CurrentTurn = *turn
	}
	if winner != nil {
		g.Winner = *winner
	}
	if start != nil {
		g.StartingPlayer = *start
	}
	for _, x := range ps {
		stats, _ := p.Stats(ctx, x.uid, "pvp")
		pl := PVPPlayer{User: x.user, Stats: stats, Board: x.board, Ready: true}
		if x.uid == uid {
			pl.Fleet = x.fleet
			g.You = pl
		} else {
			if status != "complete" {
				pl.Board = stripDates(pl.Board)
			}
			g.Opponent = pl
		}
	}
	sr, e := p.pool.Query(ctx, `SELECT shooter_id,x,y,result,coalesce(ship_kind,'') FROM pvp_shots WHERE game_id=$1 ORDER BY turn_number`, id)
	if e != nil {
		return nil, e
	}
	defer sr.Close()
	for sr.Next() {
		var shooter string
		var s game.Shot
		if e = sr.Scan(&shooter, &s.X, &s.Y, &s.Result, &s.Ship); e != nil {
			return nil, e
		}
		if shooter == uid {
			g.You.Shots = append(g.You.Shots, s)
		} else {
			g.Opponent.Shots = append(g.Opponent.Shots, s)
		}
		g.LastMove = &PVPLastMove{ShooterID: shooter, Shot: s}
	}
	if e = sr.Err(); e != nil {
		return nil, e
	}
	if status == "complete" {
		_ = p.pool.QueryRow(ctx, `SELECT rating_before,rating_after,rating_delta,hits,ships_sunk FROM pvp_results WHERE game_id=$1 AND user_id=$2`, id, uid).Scan(&g.RatingBefore, &g.RatingAfter, &g.RatingDelta, &g.Hits, &g.ShipsSunk)
		_ = p.pool.QueryRow(ctx, `SELECT rating_before,rating_after,rating_delta FROM pvp_results WHERE game_id=$1 AND user_id<>$2`, id, uid).Scan(&g.OpponentRatingBefore, &g.OpponentRatingAfter, &g.OpponentRatingDelta)
	}
	return g, nil
}

func (p *PostgresRepository) ShootPVP(ctx context.Context, uid, id string, c game.Coord) (*PVPGame, []game.Shot, error) {
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return nil, nil, e
	}
	defer tx.Rollback(ctx)
	var status, turn string
	var terminal bool
	e = tx.QueryRow(ctx, `SELECT status,current_turn_user_id,terminal_applied FROM games WHERE id=$1 AND mode='pvp' FOR UPDATE`, id).Scan(&status, &turn, &terminal)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if e != nil {
		return nil, nil, e
	}
	var target string
	var fleetJSON []byte
	e = tx.QueryRow(ctx, `SELECT them.user_id,them.fleet FROM game_players me JOIN game_players them ON them.game_id=me.game_id AND them.user_id<>me.user_id WHERE me.game_id=$1 AND me.user_id=$2`, id, uid).Scan(&target, &fleetJSON)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if e != nil {
		return nil, nil, e
	}
	if status == "complete" || terminal {
		return nil, nil, ErrGameComplete
	}
	if status != "battle" || turn != uid {
		return nil, nil, ErrNotYourTurn
	}
	var fleet []game.Ship
	_ = json.Unmarshal(fleetJSON, &fleet)
	rows, e := tx.Query(ctx, `SELECT x,y,result,coalesce(ship_kind,'') FROM pvp_shots WHERE game_id=$1 AND shooter_id=$2 ORDER BY turn_number`, id, uid)
	if e != nil {
		return nil, nil, e
	}
	var prev []game.Shot
	for rows.Next() {
		var s game.Shot
		if e = rows.Scan(&s.X, &s.Y, &s.Result, &s.Ship); e != nil {
			rows.Close()
			return nil, nil, e
		}
		prev = append(prev, s)
	}
	rows.Close()
	// Fleet coordinates are immutable; rebuild hit state from normalized shots.
	replayed := make([]game.Shot, 0, len(prev))
	for _, old := range prev {
		_, fleet, e = game.ResolveShot(fleet, replayed, old.Coord)
		if e != nil {
			return nil, nil, e
		}
		replayed = append(replayed, old)
	}
	transition, e := game.ResolvePVPTurn(status, turn, uid, target, fleet, prev, c)
	if e != nil {
		return nil, nil, e
	}
	shot := transition.Shot
	if shot.Result != "sunk" {
		shot.Ship = ""
	}
	var turnNo int
	_ = tx.QueryRow(ctx, `SELECT coalesce(max(turn_number),0)+1 FROM pvp_shots WHERE game_id=$1`, id).Scan(&turnNo)
	_, e = tx.Exec(ctx, `INSERT INTO pvp_shots(game_id,shooter_id,target_user_id,turn_number,x,y,result,ship_kind) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, uid, target, turnNo, c.X, c.Y, shot.Result, nilIfEmpty(shot.Ship))
	if e != nil {
		return nil, nil, e
	}
	if transition.WinnerID == "" {
		_, e = tx.Exec(ctx, `UPDATE games SET current_turn_user_id=$2,updated_at=now() WHERE id=$1`, id, target)
	} else {
		e = p.finishPVP(ctx, tx, id, uid, target)
	}
	if e != nil {
		return nil, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, nil, e
	}
	g, e := p.PVPGame(ctx, uid, id)
	return g, []game.Shot{shot}, e
}

func (p *PostgresRepository) finishPVP(ctx context.Context, tx pgx.Tx, gid, winner, loser string) error {
	type st struct {
		uid string
		PublicStats
	}
	ss := []st{}
	rows, e := tx.Query(ctx, `SELECT user_id,games,wins,losses,rating,shots,hits,current_streak,longest_streak,win_shots FROM mode_stats WHERE mode='pvp' AND user_id IN ($1,$2) ORDER BY user_id FOR UPDATE`, winner, loser)
	if e != nil {
		return e
	}
	for rows.Next() {
		var x st
		if e = rows.Scan(&x.uid, &x.Games, &x.Wins, &x.Losses, &x.Rating, &x.Shots, &x.Hits, &x.CurrentStreak, &x.LongestStreak, &x.WinShots); e != nil {
			rows.Close()
			return e
		}
		ss = append(ss, x)
	}
	rows.Close()
	if len(ss) != 2 {
		return ErrConflict
	}
	by := map[string]*st{ss[0].uid: &ss[0], ss[1].uid: &ss[1]}
	wr, lr := by[winner].Rating, by[loser].Rating
	wn, ln := game.Elo(wr, lr, true), game.Elo(lr, wr, false)
	for _, uid := range []string{winner, loser} {
		x := by[uid]
		var shots, hits, shipsSunk int
		_ = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE result<>'miss'),count(*) FILTER(WHERE result='sunk') FROM pvp_shots WHERE game_id=$1 AND shooter_id=$2`, gid, uid).Scan(&shots, &hits, &shipsSunk)
		won := uid == winner
		x.Games++
		x.Shots += shots
		x.Hits += hits
		if won {
			x.Wins++
			x.CurrentStreak++
			x.WinShots += shots
			if x.CurrentStreak > x.LongestStreak {
				x.LongestStreak = x.CurrentStreak
			}
			x.Rating = wn
		} else {
			x.Losses++
			x.CurrentStreak = 0
			x.Rating = ln
		}
		_, e = tx.Exec(ctx, `UPDATE mode_stats SET games=$3,wins=$4,losses=$5,rating=$6,shots=$7,hits=$8,current_streak=$9,longest_streak=$10,win_shots=$11 WHERE user_id=$1 AND mode=$2`, uid, "pvp", x.Games, x.Wins, x.Losses, x.Rating, x.Shots, x.Hits, x.CurrentStreak, x.LongestStreak, x.WinShots)
		if e != nil {
			return e
		}
		opp := winner
		if won {
			opp = loser
		}
		_, e = tx.Exec(ctx, `INSERT INTO pvp_results(game_id,user_id,opponent_id,won,shots,hits,ships_sunk,rating_before,rating_after,rating_delta) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, gid, uid, opp, won, shots, hits, shipsSunk, map[bool]int{true: wr, false: lr}[won], x.Rating, x.Rating-map[bool]int{true: wr, false: lr}[won])
		if e != nil {
			return e
		}
	}
	sid := randomSecret(9)
	_, e = tx.Exec(ctx, `UPDATE games SET status='complete',current_turn='complete',current_turn_user_id=NULL,winner_user_id=$2,winner='player',terminal_applied=true,completed_at=now(),updated_at=now() WHERE id=$1`, gid, winner)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE challenges SET status='complete' WHERE game_id=$1`, gid)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO shares(id,game_id) VALUES($1,$2)`, sid, gid)
	}
	return e
}

func (p *PostgresRepository) Battles(ctx context.Context, uid string) ([]BattleSummary, error) {
	rows, e := p.pool.Query(ctx, `SELECT g.id,g.status,ou.id,ou.login,ou.name,ou.avatar_url,g.current_turn_user_id,g.winner_user_id,c.code,g.updated_at FROM challenges c JOIN games g ON g.id=c.game_id JOIN users ou ON ou.id=CASE WHEN c.creator_id=$1 THEN c.opponent_id ELSE c.creator_id END WHERE $1 IN (c.creator_id,c.opponent_id) ORDER BY g.updated_at DESC LIMIT 100`, uid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []BattleSummary{}
	for rows.Next() {
		var x BattleSummary
		var turn, winner *string
		if e = rows.Scan(&x.ID, &x.Status, &x.Opponent.ID, &x.Opponent.Login, &x.Opponent.Name, &x.Opponent.AvatarURL, &turn, &winner, &x.ChallengeCode, &x.UpdatedAt); e != nil {
			return nil, e
		}
		x.YourTurn = turn != nil && *turn == uid
		if winner != nil {
			x.Winner = *winner
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (p *PostgresRepository) Rematch(ctx context.Context, uid, gid string, expires time.Time) (Challenge, error) {
	var other string
	e := p.pool.QueryRow(ctx, `SELECT them.user_id FROM games g JOIN game_players me ON me.game_id=g.id AND me.user_id=$2 JOIN game_players them ON them.game_id=g.id AND them.user_id<>me.user_id WHERE g.id=$1 AND g.mode='pvp' AND g.status='complete'`, gid, uid).Scan(&other)
	if errors.Is(e, pgx.ErrNoRows) {
		return Challenge{}, ErrNotFound
	}
	if e != nil {
		return Challenge{}, e
	}
	code := randomSecret(12)
	_, e = p.pool.Exec(ctx, `INSERT INTO challenges(id,code,creator_id,intended_opponent_id,status,expires_at,rematch_of_game_id) VALUES($1,$2,$3,$4,'open',$5,$6) ON CONFLICT(rematch_of_game_id) DO NOTHING`, uuid(), code, uid, other, expires, gid)
	if e != nil {
		return Challenge{}, e
	}
	var actual string
	e = p.pool.QueryRow(ctx, `SELECT code FROM challenges WHERE rematch_of_game_id=$1`, gid).Scan(&actual)
	if e != nil {
		return Challenge{}, e
	}
	return p.PublicChallenge(ctx, actual)
}
func (p *PostgresRepository) Leaderboard(ctx context.Context, limit int) ([]LeaderboardEntry, error) {
	rows, e := p.pool.Query(ctx, `SELECT u.login,u.name,u.avatar_url,s.rating,s.wins,s.games FROM mode_stats s JOIN users u ON u.id=s.user_id WHERE s.mode='pvp' AND s.games>0 ORDER BY s.rating DESC,s.wins DESC,s.games ASC,s.user_id ASC LIMIT $1`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []LeaderboardEntry{}
	for rows.Next() {
		var x LeaderboardEntry
		if e = rows.Scan(&x.Login, &x.Name, &x.AvatarURL, &x.Rating, &x.Wins, &x.Games); e != nil {
			return nil, e
		}
		x.Position = len(out) + 1
		x.Rank = decorate(PublicStats{Rating: x.Rating}).Rank
		x.WinRate = 100 * float64(x.Wins) / float64(x.Games)
		out = append(out, x)
	}
	return out, rows.Err()
}
func (p *PostgresRepository) PVPHistory(ctx context.Context, uid string, limit int) ([]PVPHistory, error) {
	rows, e := p.pool.Query(ctx, `SELECT r.game_id,s.id,u.id,u.login,u.name,u.avatar_url,r.won,r.shots,r.hits,r.rating_delta,r.created_at FROM pvp_results r JOIN users u ON u.id=r.opponent_id JOIN shares s ON s.game_id=r.game_id WHERE r.user_id=$1 ORDER BY r.created_at DESC LIMIT $2`, uid, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PVPHistory{}
	for rows.Next() {
		var x PVPHistory
		if e = rows.Scan(&x.GameID, &x.ShareID, &x.Opponent.ID, &x.Opponent.Login, &x.Opponent.Name, &x.Opponent.AvatarURL, &x.Won, &x.Shots, &x.Hits, &x.RatingDelta, &x.CompletedAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (p *PostgresRepository) PublicPVPShare(ctx context.Context, sid string) (PVPShare, error) {
	var x PVPShare
	var wr, lr int
	e := p.pool.QueryRow(ctx, `SELECT s.id,wu.id,wu.login,wu.name,wu.avatar_url,lu.id,lu.login,lu.name,lu.avatar_url,w.shots,w.hits,w.rating_delta,w.rating_after,l.shots,l.hits,l.rating_delta,l.rating_after FROM shares s JOIN games g ON g.id=s.game_id AND g.mode='pvp' AND g.status='complete' JOIN pvp_results w ON w.game_id=g.id AND w.won JOIN pvp_results l ON l.game_id=g.id AND NOT l.won JOIN users wu ON wu.id=w.user_id JOIN users lu ON lu.id=l.user_id WHERE s.id=$1`, sid).Scan(&x.ShareID, &x.Winner.ID, &x.Winner.Login, &x.Winner.Name, &x.Winner.AvatarURL, &x.Loser.ID, &x.Loser.Login, &x.Loser.Name, &x.Loser.AvatarURL, &x.WinnerResult.Shots, &x.WinnerResult.Hits, &x.WinnerResult.RatingDelta, &wr, &x.LoserResult.Shots, &x.LoserResult.Hits, &x.LoserResult.RatingDelta, &lr)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, ErrNotFound
	}
	x.Rank = decorate(PublicStats{Rating: wr}).Rank
	return x, e
}
