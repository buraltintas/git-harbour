package server

import (
	"context"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

type Challenge struct {
	Code          string      `json:"code"`
	Status        string      `json:"status"`
	ExpiresAt     time.Time   `json:"expiresAt"`
	Creator       User        `json:"creator"`
	Opponent      *User       `json:"opponent,omitempty"`
	GameID        string      `json:"gameId,omitempty"`
	CreatorReady  bool        `json:"creatorReady"`
	OpponentReady bool        `json:"opponentReady"`
	CreatorPVP    PublicStats `json:"creatorPvp"`
	OpponentPVP   PublicStats `json:"opponentPvp,omitempty"`
	// IntendedOpponentID restricts rematch links to the other participant. It is
	// never part of the public challenge representation.
	IntendedOpponentID string `json:"-"`
}
type PVPPlayer struct {
	User  User        `json:"user"`
	Stats PublicStats `json:"pvp"`
	Board []game.Cell `json:"board"`
	Fleet []game.Ship `json:"fleet,omitempty"`
	Shots []game.Shot `json:"shots"`
	Ready bool        `json:"ready"`
}
type PVPGame struct {
	ID, Ruleset, Status, CurrentTurn, Winner, ShareID, ChallengeCode string
	You, Opponent                                                    PVPPlayer
	StartingPlayer                                                   string
	RatingDelta, RatingBefore, RatingAfter, Hits, ShipsSunk          int
	OpponentRatingDelta, OpponentRatingBefore                        int
	OpponentRatingAfter                                              int
	LastMove                                                         *PVPLastMove
	UpdatedAt                                                        time.Time
}
type PVPLastMove struct {
	ShooterID string
	Shot      game.Shot
}
type BattleSummary struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	Opponent      User      `json:"opponent"`
	YourTurn      bool      `json:"yourTurn"`
	Winner        string    `json:"winner,omitempty"`
	ChallengeCode string    `json:"challengeCode"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
type PVPHistory struct {
	GameID      string    `json:"gameId"`
	ShareID     string    `json:"shareId"`
	Opponent    User      `json:"opponent"`
	Won         bool      `json:"won"`
	Shots       int       `json:"shots"`
	Hits        int       `json:"hits"`
	RatingDelta int       `json:"ratingDelta"`
	CompletedAt time.Time `json:"completedAt"`
}
type PVPShare struct {
	ShareID                   string
	Winner, Loser             User
	WinnerResult, LoserResult PVPHistory
	Rank                      string
}
type PVPShareRepository interface {
	PublicPVPShare(context.Context, string) (PVPShare, error)
}
type LeaderboardEntry struct {
	Position  int     `json:"position"`
	Login     string  `json:"login"`
	Name      string  `json:"name"`
	AvatarURL string  `json:"avatarUrl"`
	Rank      string  `json:"rank"`
	Rating    int     `json:"rating"`
	Wins      int     `json:"wins"`
	Games     int     `json:"games"`
	WinRate   float64 `json:"winRate"`
}

type OpenHarbour struct {
	Owner       User             `json:"owner"`
	PeriodStart string           `json:"periodStart"`
	Board       []game.Cell      `json:"-"`
	Deployment  []game.FleetUnit `json:"-"`
	Capacity    int              `json:"fleetCapacity"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

type AsyncPVPBattle struct {
	State      *State
	Challenger User
	Defender   User
	UpdatedAt  time.Time
}

type AsyncPVPRepository interface {
	SetOpenHarbour(context.Context, string, string, []game.Cell, []game.FleetUnit, time.Time) (OpenHarbour, error)
	OpenHarbour(context.Context, string) (OpenHarbour, error)
	CloseOpenHarbour(context.Context, string) error
	OpenHarbours(context.Context, string) ([]OpenHarbour, error)
	StartAsyncPVP(context.Context, string, string, time.Time) (*AsyncPVPBattle, error)
	AsyncPVPGame(context.Context, string, string) (*AsyncPVPBattle, error)
	AsyncPVPBattles(context.Context, string) ([]*AsyncPVPBattle, error)
	ShootAsyncPVP(context.Context, string, string, game.Coord) (*AsyncPVPBattle, []game.BattleEvent, error)
}

type PVPRepository interface {
	CreateChallenge(context.Context, string, string, time.Time) (Challenge, error)
	PublicChallenge(context.Context, string) (Challenge, error)
	AcceptChallenge(context.Context, string, string, time.Time) (Challenge, *PVPGame, error)
	CancelChallenge(context.Context, string, string, time.Time) (Challenge, error)
	ReadyChallenge(context.Context, string, string, string, []game.Cell, []game.Ship, time.Time) (Challenge, *PVPGame, error)
	PVPGame(context.Context, string, string) (*PVPGame, error)
	ShootPVP(context.Context, string, string, game.Coord) (*PVPGame, []game.Shot, error)
	Battles(context.Context, string) ([]BattleSummary, error)
	Rematch(context.Context, string, string, time.Time) (Challenge, error)
	Leaderboard(context.Context, int) ([]LeaderboardEntry, error)
	PVPHistory(context.Context, string, int) ([]PVPHistory, error)
}
