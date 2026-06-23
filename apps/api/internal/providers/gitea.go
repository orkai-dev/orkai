package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	return p.searchRepos(ctx, cfg, "")
}

func (p *giteaProvider) SearchRepos(ctx context.Context, cfg json.RawMessage, query string) ([]GitRepo, error) {
	return p.searchRepos(ctx, cfg, query)
}

type giteaRepoJSON struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

func (p *giteaProvider) searchRepos(ctx context.Context, cfg json.RawMessage, query string) ([]GitRepo, error) {
	var c giteaConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if c.APIURL == "" {
		return nil, fmt.Errorf("gitea API URL not configured")
	}

	if strings.TrimSpace(query) == "" {
		body, err := p.giteaGet(ctx, c.APIURL+"/api/v1/user/repos?limit=50", c.Token)
		if err != nil {
			return nil, err
		}
		var giteaRepos []giteaRepoJSON
		_ = json.Unmarshal(body, &giteaRepos)
		return mapGiteaRepos(giteaRepos), nil
	}

	// /repos/search spans the whole instance (including public repos owned by
	// others), so scope it to repos the authenticated user owns or contributes
	// to via uid, matching the /user/repos semantics of the empty-query path.
	apiPath := fmt.Sprintf("/api/v1/repos/search?q=%s&limit=50", url.QueryEscape(query))
	if uid, err := p.currentUserID(ctx, c.APIURL, c.Token); err == nil && uid > 0 {
		apiPath += fmt.Sprintf("&uid=%d", uid)
	}

	body, err := p.giteaGet(ctx, c.APIURL+apiPath, c.Token)
	if err != nil {
		return nil, err
	}
	var searchResult struct {
		Data []giteaRepoJSON `json:"data"`
	}
	_ = json.Unmarshal(body, &searchResult)
	return mapGiteaRepos(searchResult.Data), nil
}

// currentUserID resolves the authenticated user's numeric id, used to scope
// repository search to repos that user owns or contributes to.
func (p *giteaProvider) currentUserID(ctx context.Context, apiURL, token string) (int64, error) {
	body, err := p.giteaGet(ctx, apiURL+"/api/v1/user", token)
	if err != nil {
		return 0, err
	}
	var u struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return 0, err
	}
	return u.ID, nil
}

// giteaGet performs an authenticated GET and returns the (size-limited) body.
func (p *giteaProvider) giteaGet(ctx context.Context, url, token string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "token "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	return io.ReadAll(io.LimitReader(resp.Body, 5<<20))
}

func mapGiteaRepos(repos []giteaRepoJSON) []GitRepo {
	var out []GitRepo
	for _, r := range repos {
		out = append(out, GitRepo{
			Name:          r.Name,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return out
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
