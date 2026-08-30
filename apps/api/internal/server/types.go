package server

import (
	"context"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

type User struct {
	ID        string    `json:"-"`
	Login     string    `json:"login"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatarUrl"`
	GitHubID  int64     `json:"-"`
	JoinedAt  time.Time `json:"-"`
}
type PublicStats struct {
	Games              int     `json:"games"`
	Wins               int     `json:"wins"`
	Losses             int     `json:"losses"`
	Rating             int     `json:"rating"`
	Shots              int     `json:"shots"`
	Hits               int     `json:"hits"`
	CurrentStreak      int     `json:"currentStreak"`
	LongestStreak      int     `json:"longestStreak"`
	WinShots           int     `json:"-"`
	WinRate            float64 `json:"winRate"`
	Accuracy           float64 `json:"accuracy"`
	AverageShotsPerWin float64 `json:"averageShotsPerWin"`
	Rank               string  `json:"rank"`
}
type PublicUser struct {
	Login                     string              `json:"login"`
	Name                      string              `json:"name"`
	AvatarURL                 string              `json:"avatarUrl"`
	Solo                      PublicStats         `json:"solo"`
	PVP                       PublicStats         `json:"pvp"`
	JoinedAt                  time.Time           `json:"joinedAt"`
	PublicContributionSummary ContributionSummary `json:"publicContributionSummary"`
	PVPHistory                []PVPHistory        `json:"pvpHistory"`
}
type ContributionSummary struct {
	Total      int         `json:"total"`
	ActiveDays int         `json:"activeDays"`
	Preview    []game.Cell `json:"preview"`
}
type State struct {
	ID              string      `json:"id"`
	Status          string      `json:"status"`
	Turn            string      `json:"turn"`
	PlayerBoard     []game.Cell `json:"playerBoard"`
	EnemyBoard      []game.Cell `json:"enemyBoard"`
	PlayerFleet     []game.Ship `json:"playerFleet"`
	EnemyFleet      []game.Ship `json:"enemyFleet"`
	PlayerShots     []game.Shot `json:"playerShots"`
	AIShots         []game.Shot `json:"aiShots"`
	PlayerStart     string      `json:"playerStart"`
	EnemyStart      string      `json:"enemyStart"`
	Winner          string      `json:"winner,omitempty"`
	RatingDelta     int         `json:"ratingDelta,omitempty"`
	ShareID         string      `json:"shareId,omitempty"`
	Stats           PublicStats `json:"stats"`
	TerminalApplied bool        `json:"-"`
}

type Repository interface {
	PutOAuthState(context.Context, []byte, time.Time) error
	ConsumeOAuthState(context.Context, []byte, time.Time) error
	UpsertGitHubUser(context.Context, User, []game.Cell) (User, error)
	PutExchangeCode(context.Context, []byte, string, time.Time) error
	ConsumeExchangeCode(context.Context, []byte, time.Time) (User, error)
	CreateSession(context.Context, string, []byte, time.Time) error
	ResolveSession(context.Context, []byte, time.Time) (User, error)
	RevokeSession(context.Context, []byte) error
	Contributions(context.Context, string) ([]game.Cell, error)
	Stats(context.Context, string, string) (PublicStats, error)
	PublicUser(context.Context, string) (PublicUser, error)
	CreateGame(context.Context, string, *State) error
	Game(context.Context, string, string) (*State, error)
	Shoot(context.Context, string, string, game.Coord) (*State, []game.Shot, error)
	PublicShare(context.Context, string) (*State, User, error)
	Close()
}
