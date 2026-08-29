package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/githarbour/githarbour/apps/api/internal/game"
)

type GitHubClient interface {
	Authenticate(context.Context, string) (User, []game.Cell, error)
}
type HTTPGitHubClient struct {
	config Config
	client *http.Client
}

func NewHTTPGitHubClient(c Config) *HTTPGitHubClient {
	return &HTTPGitHubClient{config: c, client: &http.Client{Timeout: 15 * time.Second}}
}
func (g *HTTPGitHubClient) Authenticate(ctx context.Context, code string) (User, []game.Cell, error) {
	token, err := g.exchange(ctx, code)
	if err != nil {
		return User{}, nil, err
	}
	return g.viewer(ctx, token)
}
func (g *HTTPGitHubClient) exchange(ctx context.Context, code string) (string, error) {
	body, _ := json.Marshal(map[string]string{"client_id": g.config.GitHubClientID, "client_secret": g.config.GitHubClientSecret, "code": code, "redirect_uri": g.config.GitHubCallback})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.config.GitHubTokenURL, bytes.NewReader(body))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, e := g.client.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if e = json.NewDecoder(resp.Body).Decode(&out); e != nil {
		return "", e
	}
	if resp.StatusCode != 200 || out.AccessToken == "" {
		return "", fmt.Errorf("GitHub token exchange failed: %s", out.Error)
	}
	return out.AccessToken, nil
}
func (g *HTTPGitHubClient) viewer(ctx context.Context, token string) (User, []game.Cell, error) {
	from := time.Now().UTC().AddDate(-1, 0, -7).Format(time.RFC3339)
	to := time.Now().UTC().AddDate(0, 0, 1).Format(time.RFC3339)
	query := `query($from:DateTime!,$to:DateTime!){viewer{databaseId login name avatarUrl contributionsCollection(from:$from,to:$to){contributionCalendar{weeks{contributionDays{date weekday contributionCount contributionLevel}}}}}}`
	body, _ := json.Marshal(map[string]any{"query": query, "variables": map[string]string{"from": from, "to": to}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.config.GitHubGraphQLURL, bytes.NewReader(body))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, e := g.client.Do(req)
	if e != nil {
		return User{}, nil, e
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			Viewer struct {
				DatabaseID    int64  `json:"databaseId"`
				Login         string `json:"login"`
				Name          string `json:"name"`
				AvatarURL     string `json:"avatarUrl"`
				Contributions struct {
					Calendar struct {
						Weeks []struct {
							Days []struct {
								Date    string `json:"date"`
								Weekday int    `json:"weekday"`
								Count   int    `json:"contributionCount"`
								Level   string `json:"contributionLevel"`
							} `json:"contributionDays"`
						} `json:"weeks"`
					} `json:"contributionCalendar"`
				} `json:"contributionsCollection"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if e = json.NewDecoder(resp.Body).Decode(&out); e != nil {
		return User{}, nil, e
	}
	if resp.StatusCode != 200 || len(out.Errors) > 0 {
		return User{}, nil, errors.New("GitHub GraphQL request failed")
	}
	v := out.Data.Viewer
	if v.DatabaseID == 0 || v.Login == "" {
		return User{}, nil, errors.New("GitHub identity missing")
	}
	days := []game.Cell{}
	for _, w := range v.Contributions.Calendar.Weeks {
		for _, d := range w.Days {
			days = append(days, game.Cell{Date: d.Date, Weekday: d.Weekday, ContributionCount: d.Count, ContributionLevel: githubLevel(d.Level)})
		}
	}
	if len(days) > 364 {
		days = days[len(days)-364:]
	}
	if len(days) < 70 {
		return User{}, nil, errors.New("GitHub contribution calendar is incomplete")
	}
	return User{GitHubID: v.DatabaseID, Login: v.Login, Name: v.Name, AvatarURL: v.AvatarURL}, days, nil
}
func githubLevel(v string) int {
	switch v {
	case "FIRST_QUARTILE":
		return 1
	case "SECOND_QUARTILE":
		return 2
	case "THIRD_QUARTILE":
		return 3
	case "FOURTH_QUARTILE":
		return 4
	default:
		return 0
	}
}
