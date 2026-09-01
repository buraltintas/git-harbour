package server

import (
	"fmt"
	"html/template"
	"image/png"
	"net/http"
	"strings"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

func mockCells(now time.Time) []game.Cell {
	start := now.UTC().AddDate(-1, 0, 0)
	for start.Weekday() != time.Sunday {
		start = start.AddDate(0, 0, -1)
	}
	out := make([]game.Cell, 0, 364)
	for i := 0; i < 364; i++ {
		d := start.AddDate(0, 0, i)
		seed := (i*17 + i/7*11) % 23
		count := 0
		if seed > 8 {
			count = (seed*seed)%19 + 1
		}
		level := 0
		if count > 0 {
			level = 1
		}
		if count > 3 {
			level = 2
		}
		if count > 8 {
			level = 3
		}
		if count > 14 {
			level = 4
		}
		out = append(out, game.Cell{Date: d.Format("2006-01-02"), Weekday: int(d.Weekday()), ContributionCount: count, ContributionLevel: level})
	}
	return out
}
func (s *Server) publicUserJSON(w http.ResponseWriter, r *http.Request) {
	u, e := s.repo.PublicUser(r.Context(), r.PathValue("login"))
	if e != nil {
		writeError(w, 404, "user_not_found", "This GitHub user has not joined GitHarbour.")
		return
	}
	writeJSON(w, 200, u)
}

var profileTemplate = template.Must(template.New("profile").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>@{{.User.Login}} on GitHarbour — {{.User.Solo.Rank}}</title><meta name="description" content="{{.User.Solo.Wins}} wins · {{.User.Solo.Losses}} losses · {{.User.Solo.Rating}} rating · Your activity builds your fleet."><link rel="canonical" href="{{.Canonical}}"><meta property="og:title" content="@{{.User.Login}} on GitHarbour — {{.User.Solo.Rank}}"><meta property="og:description" content="{{.User.Solo.Wins}} wins · {{.User.Solo.Losses}} losses · {{.User.Solo.Rating}} rating"><meta property="og:image" content="{{.Widget}}?theme=dark"><meta property="og:type" content="profile"><meta property="og:url" content="{{.Canonical}}"><meta name="twitter:card" content="summary_large_image"><style>body{font:16px system-ui;margin:0;background:#f6f8fa;color:#1f2328}main{max-width:760px;margin:64px auto;padding:32px;background:white;border:1px solid #d0d7de;border-radius:8px}header{display:flex;gap:18px;align-items:center}img{width:72px;height:72px;border-radius:50%}h1{margin:0}small,p{color:#656d76}.stats{display:grid;grid-template-columns:repeat(2,1fr);gap:16px;margin:28px 0}.card{border:1px solid #d0d7de;border-radius:6px;padding:18px}.card strong{font-size:28px;display:block}a{color:#0969da}@media(max-width:600px){main{margin:0;border:0;border-radius:0}.stats{grid-template-columns:1fr}}</style></head><body><main><header><img src="{{.User.AvatarURL}}" alt=""><div><small>GitHarbour developer</small><h1>@{{.User.Login}}</h1><p>{{.User.Name}}</p></div></header><section class="stats"><div class="card"><small>Arcade · {{.User.Solo.Rank}}</small><strong>{{.User.Solo.Rating}}</strong><span>{{.User.Solo.Wins}} wins · {{.User.Solo.Losses}} losses · {{printf "%.0f" .User.Solo.Accuracy}}% accuracy</span></div><div class="card"><small>PvP</small>{{if .User.PVP.Games}}<strong>{{.User.PVP.Rating}}</strong><span>{{.User.PVP.Wins}} wins</span>{{else}}<strong>—</strong><span>Refit in progress</span>{{end}}</div></section><h2>Contribution harbour</h2><p>{{.User.PublicContributionSummary.ActiveDays}} active days in the public preview · {{.User.PublicContributionSummary.Total}} contributions.</p><blockquote>“Your activity builds your fleet.”</blockquote><p><a href="https://github.com/{{.User.Login}}">GitHub account</a> · <a href="{{.AppProfile}}">Interactive profile</a> · <a href="{{.Widget}}">README widget</a></p></main></body></html>`))

func (s *Server) publicUserHTML(w http.ResponseWriter, r *http.Request) {
	u, e := s.repo.PublicUser(r.Context(), r.PathValue("login"))
	if e != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(404)
		fmt.Fprint(w, `<!doctype html><html><head><title>Developer not found · GitHarbour</title></head><body><main><h1>Developer not found</h1><p>This GitHub user has not joined GitHarbour.</p></main></body></html>`)
		return
	}
	canonical := s.cfg.PublicAPIURL + "/u/" + u.Login
	widget := s.cfg.PublicAPIURL + "/widgets/" + u.Login + ".svg"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = profileTemplate.Execute(w, map[string]any{"User": u, "Canonical": canonical, "Widget": widget, "AppProfile": strings.TrimSuffix(s.cfg.WebAppURL, "/") + "/u/" + u.Login})
}
func (s *Server) widget(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if !strings.HasSuffix(strings.ToLower(file), ".svg") {
		http.NotFound(w, r)
		return
	}
	login := file[:len(file)-4]
	u, e := s.repo.PublicUser(r.Context(), login)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	theme := r.URL.Query().Get("theme")
	bg, text, muted, border, accent := "#ffffff", "#1f2328", "#656d76", "#d0d7de", "#1f883d"
	if theme == "dark" || theme == "" {
		bg, text, muted, border, accent = "#0d1117", "#f0f6fc", "#8c959f", "#30363d", "#3fb950"
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="140" viewBox="0 0 480 140" role="img" aria-label="GitHarbour stats for @%s"><rect x=".5" y=".5" width="479" height="139" rx="8" fill="%s" stroke="%s"/><path d="M24 20h20v20H24zM29 25h10v10H29z" fill="%s"/><text x="54" y="35" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="18" font-weight="700">GitHarbour · @%s</text><text x="24" y="68" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="14">Arcade · %s · %d rating · %dW / %dL</text><text x="24" y="95" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="14">%d games · %.0f%% accuracy</text><text x="24" y="122" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="12">Your activity builds your fleet.</text></svg>`, escape(u.Login), bg, border, accent, text, escape(u.Login), muted, escape(u.Solo.Rank), u.Solo.Rating, u.Solo.Wins, u.Solo.Losses, text, u.Solo.Games, u.Solo.Accuracy, accent)
}
func (s *Server) shareHTML(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.repo.(PVPShareRepository); ok {
		if x, e := p.PublicPVPShare(r.Context(), r.PathValue("id")); e == nil {
			title := fmt.Sprintf("@%s defeated @%s on GitHarbour", x.Winner.Login, x.Loser.Login)
			desc := fmt.Sprintf("%d shots · %+.0d rating · %s", x.WinnerResult.Shots, x.WinnerResult.RatingDelta, x.Rank)
			imageURL := s.cfg.PublicAPIURL + "/share/games/" + x.ShareID + ".png"
			canonical := s.cfg.PublicAPIURL + "/s/" + x.ShareID
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>%s</title><meta property="og:title" content="@%s vs @%s — GitHarbour"><meta property="og:description" content="%s"><meta property="og:image" content="%s"><meta property="og:url" content="%s"><meta name="twitter:card" content="summary_large_image"></head><body><main><h1>%s</h1><p>%s</p></main></body></html>`, escape(title), escape(x.Winner.Login), escape(x.Loser.Login), escape(desc), escape(imageURL), escape(canonical), escape(title), escape(desc))
			return
		}
	}
	g, u, e := s.repo.PublicShare(r.Context(), r.PathValue("id"))
	if e != nil {
		http.NotFound(w, r)
		return
	}
	result, title := "Archived", fmt.Sprintf("@%s — archived GitHarbour history hunt", u.Login)
	if len(g.PlayerBoard) == game.BoardCells && len(g.EnemyBoard) == game.BoardCells {
		result = "Defeat"
		if g.Winner == "player" {
			result = "Victory"
		}
		title = fmt.Sprintf("@%s — %s on GitHarbour", u.Login, result)
	}
	description := "Archived pre-launch GitHarbour result."
	if g.Ruleset == game.ContributionFleetRuleset {
		playerActions, _, playerMisses, _, playerClashes, _, playerWins, _ := fleetMetrics(g.FleetActions)
		description = fmt.Sprintf("%s · %d actions · %d clashes (%d won) · %d misses · %s–%s vs %s–%s", result, playerActions, playerClashes, playerWins, playerMisses, g.PlayerStart, dateEnd(g.PlayerStart), g.EnemyStart, dateEnd(g.EnemyStart))
	}
	if (g.Ruleset == "contribution_targets_v2" || g.Ruleset == game.ContributionBattleshipRuleset) && len(g.PlayerTargetShots) > 0 {
		hits, _ := targetShotCounts(g.PlayerTargetShots)
		description = fmt.Sprintf("%s · %d shots · %.0f%% accuracy · %s–%s vs %s–%s", result, len(g.PlayerTargetShots), 100*float64(hits)/float64(len(g.PlayerTargetShots)), g.PlayerStart, dateEnd(g.PlayerStart), g.EnemyStart, dateEnd(g.EnemyStart))
	}
	image := s.cfg.PublicAPIURL + "/share/games/" + g.ShareID + ".png"
	canonical := s.cfg.PublicAPIURL + "/s/" + g.ShareID
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title><meta name="description" content="%s"><link rel="canonical" href="%s"><meta property="og:title" content="%s"><meta property="og:description" content="%s"><meta property="og:image" content="%s"><meta property="og:type" content="website"><meta property="og:url" content="%s"><meta name="twitter:card" content="summary_large_image"></head><body><main><h1>GitHarbour battle result</h1><p>%s</p><p><a href="%s/u/%s">View @%s’s GitHarbour profile</a></p></main></body></html>`, escape(title), escape(description), escape(canonical), escape(title), escape(description), escape(image), escape(canonical), escape(description), escape(s.cfg.PublicAPIURL), escape(u.Login), escape(u.Login))
}
func (s *Server) sharePNG(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if !strings.HasSuffix(file, ".png") {
		http.NotFound(w, r)
		return
	}
	sid := strings.TrimSuffix(file, ".png")
	if p, ok := s.repo.(PVPShareRepository); ok {
		if share, e := p.PublicPVPShare(r.Context(), sid); e == nil {
			img, e := renderPVPShareCard(share)
			if e != nil {
				writeError(w, 500, "share_image_failed", "Could not render share image.")
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			_ = png.Encode(w, img)
			return
		}
	}
	g, u, e := s.repo.PublicShare(r.Context(), sid)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	img, e := renderSoloShareCard(g, u)
	if e != nil {
		writeError(w, 500, "share_image_failed", "Could not render share image.")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = png.Encode(w, img)
}
