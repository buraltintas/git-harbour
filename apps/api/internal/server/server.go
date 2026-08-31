package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg    Config
	repo   Repository
	github GitHubClient
	now    func() time.Time
	limits *requestLimiter
}

func New(ctx context.Context) (*Server, error) {
	cfg, e := ConfigFromEnv()
	if e != nil {
		return nil, e
	}
	return NewWithConfig(ctx, cfg, nil, nil)
}
func NewWithConfig(ctx context.Context, cfg Config, repo Repository, gh GitHubClient) (*Server, error) {
	if repo == nil {
		if cfg.DatabaseURL != "" {
			pc, e := cfg.PoolConfig()
			if e != nil {
				return nil, e
			}
			pool, e := pgxpool.NewWithConfig(ctx, pc)
			if e != nil {
				return nil, e
			}
			if e = pool.Ping(ctx); e != nil {
				pool.Close()
				return nil, e
			}
			if e = checkSchema(ctx, pool); e != nil {
				pool.Close()
				return nil, e
			}
			repo = NewPostgresRepository(pool)
		} else {
			if cfg.AppEnv == "production" {
				return nil, errors.New("DATABASE_URL is required in production")
			}
			repo = NewMemoryRepository()
		}
	}
	if gh == nil {
		gh = NewHTTPGitHubClient(cfg)
	}
	// A Solo board can legitimately require all 70 cells. Keep enough room for
	// setup/retries while preserving a conservative per-session abuse ceiling.
	return &Server{cfg: cfg, repo: repo, github: gh, now: func() time.Time { return time.Now().UTC() }, limits: newRequestLimiter(120, time.Minute)}, nil
}
func checkSchema(ctx context.Context, p *pgxpool.Pool) error {
	var ok bool
	e := p.QueryRow(ctx, `SELECT to_regclass('public.auth_sessions') IS NOT NULL AND to_regclass('public.games') IS NOT NULL AND to_regclass('public.pvp_shots') IS NOT NULL AND to_regclass('public.pvp_results') IS NOT NULL AND to_regclass('public.ruleset_mode_stats') IS NOT NULL AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='games' AND column_name='ruleset') AND EXISTS(SELECT 1 FROM schema_migrations WHERE version='008_contribution_fleet_v3.sql')`).Scan(&ok)
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("database schema is not current; run go run ./cmd/migrate up")
	}
	return nil
}
func (s *Server) Close() { s.repo.Close() }
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", s.health)
	m.HandleFunc("GET /auth/github/start", s.oauthStart)
	m.HandleFunc("GET /auth/github/callback", s.oauthCallback)
	m.HandleFunc("POST /auth/exchange", s.exchange)
	m.HandleFunc("POST /auth/logout", s.logout)
	m.HandleFunc("POST /v1/dev/session", s.devSession)
	m.HandleFunc("GET /v1/me", s.me)
	m.HandleFunc("GET /v1/me/contributions", s.contributions)
	m.HandleFunc("POST /v1/games/solo", s.createGame)
	m.HandleFunc("GET /v1/games/{id}", s.getGame)
	m.HandleFunc("POST /v1/games/{id}/deployment", s.deployFleet)
	m.HandleFunc("POST /v1/games/{id}/actions", s.fleetAction)
	m.HandleFunc("POST /v1/games/{id}/shots", s.shot)
	m.HandleFunc("GET /v1/public/challenges/{code}", s.publicChallenge)
	m.HandleFunc("POST /v1/challenges", s.createChallenge)
	m.HandleFunc("POST /v1/challenges/{code}/accept", s.acceptChallenge)
	m.HandleFunc("POST /v1/challenges/{code}/cancel", s.cancelChallenge)
	m.HandleFunc("POST /v1/challenges/{code}/ready", s.readyChallenge)
	m.HandleFunc("GET /v1/battles", s.battles)
	m.HandleFunc("POST /v1/games/{id}/rematch", s.rematch)
	m.HandleFunc("GET /v1/public/leaderboards/pvp", s.leaderboard)
	m.HandleFunc("GET /v1/public/users/{login}", s.publicUserJSON)
	m.HandleFunc("GET /u/{login}", s.publicUserHTML)
	m.HandleFunc("GET /widgets/{file}", s.widget)
	m.HandleFunc("GET /share/users/{file}", s.widget)
	m.HandleFunc("GET /s/{id}", s.shareHTML)
	m.HandleFunc("GET /share/games/{file}", s.sharePNG)
	return s.cors(s.rateLimitPVP(m))
}
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(s.cfg.WebOrigins, ",") {
		allowed[strings.TrimSpace(o)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			if origin != "" && !allowed[origin] {
				writeError(w, 403, "origin_not_allowed", "Origin is not allowed.")
				return
			}
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok", "environment": s.cfg.AppEnv})
}
func (s *Server) currentUser(r *http.Request) (User, error) {
	token := bearer(r)
	if token == "" {
		return User{}, ErrUnauthorized
	}
	return s.repo.ResolveSession(r.Context(), digest(token), s.now())
}
func (s *Server) needUser(w http.ResponseWriter, r *http.Request) (User, bool) {
	u, e := s.currentUser(r)
	if e != nil {
		writeError(w, 401, "unauthorized", "A valid GitHarbour session is required.")
		return User{}, false
	}
	return u, true
}
