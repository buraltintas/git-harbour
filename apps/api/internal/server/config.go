package server

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	AppEnv, DatabaseURL, WebOrigins, PublicAPIURL, WebAppURL, GitHubClientID, GitHubClientSecret, GitHubCallback, GitHubWebCallback, GitHubAPIURL, GitHubGraphQLURL, GitHubTokenURL string
	DevAuth                                                                                                                                                                         bool
	DBMaxConns, DBMinConns                                                                                                                                                          int32
	DBMaxConnLifetime                                                                                                                                                               time.Duration
}

func ConfigFromEnv() (Config, error) {
	c := Config{AppEnv: value("APP_ENV", "development"), DatabaseURL: os.Getenv("DATABASE_URL"), WebOrigins: value("WEB_ORIGIN", "http://localhost:5173"), PublicAPIURL: value("PUBLIC_API_URL", "http://localhost:8080"), WebAppURL: value("WEB_APP_URL", "http://localhost:5173/#/"), GitHubClientID: os.Getenv("GITHUB_CLIENT_ID"), GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"), GitHubCallback: value("GITHUB_OAUTH_CALLBACK", "http://localhost:8080/auth/github/callback"), GitHubWebCallback: value("GITHUB_WEB_CALLBACK", "http://localhost:5173/#/auth/callback"), GitHubAPIURL: value("GITHUB_API_URL", "https://api.github.com"), GitHubGraphQLURL: value("GITHUB_GRAPHQL_URL", "https://api.github.com/graphql"), GitHubTokenURL: value("GITHUB_TOKEN_URL", "https://github.com/login/oauth/access_token"), DBMaxConns: int32(integer("DB_MAX_CONNS", 5)), DBMinConns: int32(integer("DB_MIN_CONNS", 0)), DBMaxConnLifetime: duration("DB_MAX_CONN_LIFETIME", 30*time.Minute)}
	c.DevAuth = (c.AppEnv == "development" || c.AppEnv == "test") && strings.EqualFold(os.Getenv("GITHARBOUR_DEV_AUTH"), "true")
	if c.AppEnv != "development" && c.AppEnv != "test" && c.AppEnv != "production" {
		return c, errors.New("APP_ENV must be development, test, or production")
	}
	if c.AppEnv == "production" && c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required in production")
	}
	return c, nil
}
func (c Config) PoolConfig() (*pgxpool.Config, error) {
	p, e := pgxpool.ParseConfig(c.DatabaseURL)
	if e != nil {
		return nil, e
	}
	p.MaxConns = c.DBMaxConns
	p.MinConns = c.DBMinConns
	p.MaxConnLifetime = c.DBMaxConnLifetime
	return p, nil
}
func value(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func integer(k string, d int) int {
	v, e := strconv.Atoi(os.Getenv(k))
	if e != nil {
		return d
	}
	return v
}
func duration(k string, d time.Duration) time.Duration {
	v, e := time.ParseDuration(os.Getenv(k))
	if e != nil {
		return d
	}
	return v
}
