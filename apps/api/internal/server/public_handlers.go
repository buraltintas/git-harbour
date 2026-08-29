package server

import (
	"encoding/base64"
	"fmt"
	"html/template"
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

var profileTemplate = template.Must(template.New("profile").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>@{{.User.Login}} on GitHarbour — {{.User.Solo.Rank}}</title><meta name="description" content="{{.User.Solo.Wins}} Solo wins · {{.User.Solo.Rating}} rating · Your GitHub history is a battlefield."><link rel="canonical" href="{{.Canonical}}"><meta property="og:title" content="@{{.User.Login}} on GitHarbour — {{.User.Solo.Rank}}"><meta property="og:description" content="{{.User.Solo.Wins}} Solo wins · {{.User.Solo.Rating}} rating · Your GitHub history is a battlefield."><meta property="og:image" content="{{.Widget}}?theme=dark"><meta property="og:type" content="profile"><meta property="og:url" content="{{.Canonical}}"><meta name="twitter:card" content="summary_large_image"><style>body{font:16px system-ui;margin:0;background:#f6f8fa;color:#1f2328}main{max-width:760px;margin:64px auto;padding:32px;background:white;border:1px solid #d0d7de;border-radius:8px}header{display:flex;gap:18px;align-items:center}img{width:72px;height:72px;border-radius:50%}h1{margin:0}small,p{color:#656d76}.stats{display:grid;grid-template-columns:repeat(2,1fr);gap:16px;margin:28px 0}.card{border:1px solid #d0d7de;border-radius:6px;padding:18px}.card strong{font-size:28px;display:block}a{color:#0969da}@media(max-width:600px){main{margin:0;border:0;border-radius:0}.stats{grid-template-columns:1fr}}</style></head><body><main><header><img src="{{.User.AvatarURL}}" alt=""><div><small>GitHarbour developer</small><h1>@{{.User.Login}}</h1><p>{{.User.Name}}</p></div></header><section class="stats"><div class="card"><small>Solo · {{.User.Solo.Rank}}</small><strong>{{.User.Solo.Rating}}</strong><span>{{.User.Solo.Wins}} wins · {{printf "%.0f" .User.Solo.WinRate}}% win rate · {{printf "%.0f" .User.Solo.Accuracy}}% accuracy</span></div><div class="card"><small>PvP</small>{{if .User.PVP.Games}}<strong>{{.User.PVP.Rating}}</strong><span>{{.User.PVP.Wins}} wins</span>{{else}}<strong>—</strong><span>Not played yet</span>{{end}}</div></section><h2>Contribution harbour</h2><p>{{.User.PublicContributionSummary.ActiveDays}} active days in the public preview · {{.User.PublicContributionSummary.Total}} contributions.</p><blockquote>“Your GitHub history is a battlefield.”</blockquote><p><a href="https://github.com/{{.User.Login}}">GitHub account</a> · <a href="{{.AppProfile}}">Interactive profile</a> · <a href="{{.Widget}}">README widget</a></p></main></body></html>`))

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
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="140" viewBox="0 0 480 140" role="img" aria-label="GitHarbour stats for @%s"><rect x=".5" y=".5" width="479" height="139" rx="8" fill="%s" stroke="%s"/><path d="M24 25h20v20H24zM29 30h10v10H29z" fill="%s"/><text x="54" y="40" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="18" font-weight="700">GitHarbour</text><text x="24" y="69" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="14">@%s · %s</text><text x="24" y="97" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="16" font-weight="600">%d Solo rating</text><text x="194" y="97" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="14">%d wins · %.0f%% accuracy</text><text x="24" y="122" fill="%s" font-family="system-ui,-apple-system,sans-serif" font-size="12">Your GitHub history is a battlefield.</text></svg>`, escape(u.Login), bg, border, accent, text, muted, escape(u.Login), escape(u.Solo.Rank), text, u.Solo.Rating, text, u.Solo.Wins, u.Solo.Accuracy, accent)
}
func (s *Server) shareHTML(w http.ResponseWriter, r *http.Request) {
	g, u, e := s.repo.PublicShare(r.Context(), r.PathValue("id"))
	if e != nil {
		http.NotFound(w, r)
		return
	}
	title := fmt.Sprintf("@%s battled their GitHub history on GitHarbour", u.Login)
	description := fmt.Sprintf("%s · Solo rating %d · Your GitHub history is a battlefield.", strings.Title(g.Winner), g.Stats.Rating)
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
	if _, _, e := s.repo.PublicShare(r.Context(), sid); e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M/wHwAFAgIAn2kOCQAAAABJRU5ErkJggg==")
	_, _ = w.Write(b)
}
