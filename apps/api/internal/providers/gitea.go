package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// giteaProvider implements GitProvider for self-hosted Gitea instances. The
// instance URL must be supplied via api_url.
type giteaProvider struct {
	client *http.Client
}

func newGiteaProvider(client *http.Client) *giteaProvider {
	return &giteaProvider{client: client}
}

func (p *giteaProvider) Name() string { return "gitea" }

type giteaConfig struct {
	Token  string `json:"token"`
	APIURL string `json:"api_url"`
}

func (p *giteaProvider) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	var c giteaConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return false, "invalid config", nil
	}
	if c.APIURL == "" {
		return false, "gitea API URL not configured", nil
	}
	apiURL := fmt.Sprintf("%s/api/v1/user", c.APIURL)
	return testGitBearer(ctx, p.client, apiURL, c.Token)
}

func (p *giteaProvider) Refresh(ctx context.Context, cfg json.RawMessage) (json.RawMessage, bool, error) {
	return cfg, false, nil
}

func (p *giteaProvider) CloneToken(ctx context.Context, repoURL string, cfg json.RawMessage) (string, error) {
	return storedToken(cfg)
}

func (p *giteaProvider) ListRepos(ctx context.Context, cfg json.RawMessage) ([]GitRepo, error) {
	var c giteaConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if c.APIURL == "" {
		return nil, fmt.Errorf("gitea API URL not configured")
	}

	req, _ := http.NewRequest("GET", c.APIURL+"/api/v1/user/repos?limit=50", nil)
	req.Header.Set("Authorization", "token "+c.Token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var giteaRepos []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	_ = json.Unmarshal(body, &giteaRepos)

	var repos []GitRepo
	for _, r := range giteaRepos {
		repos = append(repos, GitRepo{
			Name:          r.Name,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return repos, nil
}

// VerifyWebhook is not yet supported for Gitea (no Gitea webhook endpoint is
// wired up). It always reports unverified.
func (p *giteaProvider) VerifyWebhook(secret string, body []byte, headers http.Header) bool {
	return false
}

func init() {
	registerGit(func(d factoryDeps) GitProvider {
		return newGiteaProvider(d.client)
	})
}
