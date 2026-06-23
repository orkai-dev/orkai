package providers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// gitlabProvider implements GitProvider for GitLab (gitlab.com and
// self-managed via api_url).
type gitlabProvider struct {
	client *http.Client
}

func newGitLabProvider(client *http.Client) *gitlabProvider {
	return &gitlabProvider{client: client}
}

func (p *gitlabProvider) Name() string { return "gitlab" }

type gitlabConfig struct {
	Token  string `json:"token"`
	APIURL string `json:"api_url"`
}

func (p *gitlabProvider) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	var c gitlabConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return false, "invalid config", nil
	}
	apiURL := c.APIURL
	if apiURL == "" {
		apiURL = "https://gitlab.com/api/v4/user"
	}
	return testGitBearer(ctx, p.client, apiURL, c.Token)
}

func (p *gitlabProvider) Refresh(ctx context.Context, cfg json.RawMessage) (json.RawMessage, bool, error) {
	return cfg, false, nil
}

func (p *gitlabProvider) CloneToken(ctx context.Context, repoURL string, cfg json.RawMessage) (string, error) {
	return storedToken(cfg)
}

func (p *gitlabProvider) ListRepos(ctx context.Context, cfg json.RawMessage) ([]GitRepo, error) {
	return p.searchRepos(ctx, cfg, "")
}

func (p *gitlabProvider) SearchRepos(ctx context.Context, cfg json.RawMessage, query string) ([]GitRepo, error) {
	return p.searchRepos(ctx, cfg, query)
}

func (p *gitlabProvider) searchRepos(ctx context.Context, cfg json.RawMessage, query string) ([]GitRepo, error) {
	var c gitlabConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	apiURL := c.APIURL
	if apiURL == "" {
		apiURL = "https://gitlab.com"
	}

	params := fmt.Sprintf("/api/v4/projects?membership=true&per_page=50&order_by=updated_at&search=%s", url.QueryEscape(query))
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL+params, nil)
	req.Header.Set("PRIVATE-TOKEN", c.Token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var projects []struct {
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		HTTPURLToRepo     string `json:"http_url_to_repo"`
		DefaultBranch     string `json:"default_branch"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	_ = json.Unmarshal(body, &projects)

	var repos []GitRepo
	for _, pr := range projects {
		repos = append(repos, GitRepo{
			Name:          pr.Name,
			FullName:      pr.PathWithNamespace,
			CloneURL:      pr.HTTPURLToRepo,
			DefaultBranch: pr.DefaultBranch,
		})
	}
	return repos, nil
}

// VerifyWebhook validates a GitLab webhook via the X-Gitlab-Token header.
func (p *gitlabProvider) VerifyWebhook(secret string, body []byte, headers http.Header) bool {
	token := headers.Get("X-Gitlab-Token")
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}

// testGitBearer performs a GET against a "current user" endpoint using a bearer
// token and interprets the result as a connectivity/auth check. Shared by the
// bearer-style git providers.
func testGitBearer(ctx context.Context, client *http.Client, apiURL, token string) (bool, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, "GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return false, "connection failed: " + err.Error(), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 200 {
		return true, "authenticated successfully", nil
	}
	return false, fmt.Sprintf("authentication failed (HTTP %d)", resp.StatusCode), nil
}

// storedToken extracts the "token" field from a resource config, used as the
// clone credential for providers without a fresh-token mechanism.
func storedToken(cfg json.RawMessage) (string, error) {
	var c struct {
		Token string `json:"token"`
	}
	if len(cfg) > 0 && json.Unmarshal(cfg, &c) == nil && c.Token != "" {
		return c.Token, nil
	}
	return "", fmt.Errorf("no token available")
}

func init() {
	registerGit(func(d factoryDeps) GitProvider {
		return newGitLabProvider(d.client)
	})
}
