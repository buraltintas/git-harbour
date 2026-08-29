package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	if s.cfg.GitHubClientID == "" || s.cfg.GitHubClientSecret == "" {
		writeError(w, 503, "oauth_not_configured", "GitHub OAuth is not configured.")
		return
	}
	state := randomSecret(32)
	if e := s.repo.PutOAuthState(r.Context(), digest(state), s.now().Add(10*time.Minute)); e != nil {
		writeError(w, 500, "oauth_state_failed", "Could not begin GitHub sign-in.")
		return
	}
	q := url.Values{"client_id": {s.cfg.GitHubClientID}, "redirect_uri": {s.cfg.GitHubCallback}, "state": {state}}
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}
func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	state, code := r.URL.Query().Get("state"), r.URL.Query().Get("code")
	if state == "" || code == "" || s.repo.ConsumeOAuthState(r.Context(), digest(state), s.now()) != nil {
		writeError(w, 400, "invalid_oauth_state", "The GitHub sign-in state is invalid, expired, or already used.")
		return
	}
	u, days, e := s.github.Authenticate(r.Context(), code)
	if e != nil {
		writeError(w, 502, "github_auth_failed", "GitHub sign-in could not be completed.")
		return
	}
	u, e = s.repo.UpsertGitHubUser(r.Context(), u, days)
	if e != nil {
		writeError(w, 500, "user_import_failed", "GitHub profile could not be imported.")
		return
	}
	exchange := randomSecret(32)
	if e = s.repo.PutExchangeCode(r.Context(), digest(exchange), u.ID, s.now().Add(90*time.Second)); e != nil {
		writeError(w, 500, "exchange_failed", "Could not complete sign-in.")
		return
	}
	separator := "?"
	if strings.Contains(s.cfg.GitHubWebCallback, "?") {
		separator = "&"
	}
	http.Redirect(w, r, s.cfg.GitHubWebCallback+separator+"code="+url.QueryEscape(exchange), http.StatusFound)
}
func (s *Server) exchange(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Code string `json:"code"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.Code == "" {
		writeError(w, 400, "invalid_exchange_code", "Exchange code is required.")
		return
	}
	u, e := s.repo.ConsumeExchangeCode(r.Context(), digest(q.Code), s.now())
	if e != nil {
		writeError(w, 401, "invalid_exchange_code", "Exchange code is invalid, expired, or already used.")
		return
	}
	token := randomSecret(32)
	if e = s.repo.CreateSession(r.Context(), u.ID, digest(token), s.now().Add(30*24*time.Hour)); e != nil {
		writeError(w, 500, "session_failed", "Could not create a session.")
		return
	}
	writeJSON(w, 200, map[string]any{"token": token, "user": userDTO(u)})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token := bearer(r)
	if token == "" || s.repo.RevokeSession(r.Context(), digest(token)) != nil {
		writeError(w, 401, "unauthorized", "A valid GitHarbour session is required.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) devSession(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.DevAuth {
		http.NotFound(w, r)
		return
	}
	days := mockCells(s.now())
	u, e := s.repo.UpsertGitHubUser(r.Context(), User{GitHubID: 583231, Login: "octocat", Name: "The Octocat", AvatarURL: "https://github.com/octocat.png"}, days)
	if e != nil {
		writeError(w, 500, "dev_session_failed", "Could not create development user.")
		return
	}
	token := randomSecret(32)
	if e = s.repo.CreateSession(r.Context(), u.ID, digest(token), s.now().Add(24*time.Hour)); e != nil {
		writeError(w, 500, "dev_session_failed", "Could not create development session.")
		return
	}
	writeJSON(w, 200, map[string]any{"token": token, "user": userDTO(u)})
}
func userDTO(u User) map[string]any {
	return map[string]any{"login": u.Login, "name": u.Name, "avatarUrl": u.AvatarURL, "joinedAt": u.JoinedAt}
}
