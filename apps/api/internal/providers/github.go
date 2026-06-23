package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// githubProvider implements GitProvider for GitHub, including GitHub App
// installation tokens (used for cloning private repos during builds) and OAuth
// token refresh.
type githubProvider struct {
	settings SettingsGetter
	client   *http.Client
	logger   *slog.Logger
}

func newGitHubProvider(settings SettingsGetter, client *http.Client, logger *slog.Logger) *githubProvider {
	return &githubProvider{settings: settings, client: client, logger: logger}
}

func (p *githubProvider) Name() string { return "github" }

type githubConfig struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	APIURL       string `json:"api_url"`
	Org          string `json:"org"`
	Username     string `json:"username"`
}

func (p *githubProvider) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	var c githubConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return false, "invalid config", nil
	}
	apiURL := c.APIURL
	if apiURL == "" {
		apiURL = "https://api.github.com/user"
	}
	return testGitBearer(ctx, p.client, apiURL, c.Token)
}

func (p *githubProvider) Refresh(ctx context.Context, cfg json.RawMessage) (json.RawMessage, bool, error) {
	var c githubConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		// Leave invalid config untouched; ListRepos will report it.
		return cfg, false, nil
	}
	if c.RefreshToken == "" || c.ExpiresAt == "" {
		return cfg, false, nil
	}
	expiresAt, _ := time.Parse(time.RFC3339, c.ExpiresAt)
	// Refresh 5 minutes before expiry.
	if time.Now().Before(expiresAt.Add(-5 * time.Minute)) {
		return cfg, false, nil
	}
	updated, err := p.refreshToken(ctx, cfg, c.RefreshToken)
	if err != nil {
		return cfg, false, err
	}
	if p.logger != nil {
		p.logger.Info("GitHub token refreshed")
	}
	return updated, true, nil
}

// refreshToken exchanges a refresh_token for a new access_token and returns the
// merged config. Persisting the returned config is the caller's responsibility.
func (p *githubProvider) refreshToken(ctx context.Context, cfg json.RawMessage, refreshToken string) (json.RawMessage, error) {
	clientID, _ := p.settings.Get(ctx, "github_app_client_id")
	clientSecret, _ := p.settings.Get(ctx, "github_app_client_secret")
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("GitHub App not configured")
	}

	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid refresh response: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("refresh failed: %s", result.Error)
	}

	var configMap map[string]string
	_ = json.Unmarshal(cfg, &configMap)
	if configMap == nil {
		configMap = make(map[string]string)
	}
	configMap["token"] = result.AccessToken
	if result.RefreshToken != "" {
		configMap["refresh_token"] = result.RefreshToken
	}
	if result.ExpiresIn > 0 {
		configMap["expires_at"] = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
	newConfig, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}
	return newConfig, nil
}

func (p *githubProvider) ListRepos(ctx context.Context, cfg json.RawMessage) ([]GitRepo, error) {
	var c githubConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	repos, err := p.listRepos(c.Token, c.Org)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w — the token may have expired, try reconnecting in Resources", err)
	}
	return repos, nil
}

func (p *githubProvider) SearchRepos(ctx context.Context, cfg json.RawMessage, query string) ([]GitRepo, error) {
	var c githubConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	repos, err := p.searchRepos(c.Token, c.Org, c.Username, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search repositories: %w — the token may have expired, try reconnecting in Resources", err)
	}
	return repos, nil
}

const githubSearchPerPage = 50

type githubRepoJSON struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

func (p *githubProvider) searchRepos(token, org, username, query string) ([]GitRepo, error) {
	if strings.TrimSpace(query) == "" {
		return p.listReposFirstPage(token, org)
	}
	return p.searchReposAPI(token, org, username, query)
}

func (p *githubProvider) listReposFirstPage(token, org string) ([]GitRepo, error) {
	installationID, err := p.findInstallation(token, org)
	if err != nil || installationID == 0 {
		return p.listReposFallbackPage(token, org, 1)
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/user/installations/%d/repositories?per_page=%d&page=1",
		installationID, githubSearchPerPage,
	)
	status, body, err := p.getJSON(apiURL, token)
	if err != nil {
		return nil, err
	}
	if status == 401 || status == 403 {
		return nil, fmt.Errorf("GitHub token expired or revoked (HTTP %d) — please reconnect in Resources", status)
	}

	var result struct {
		Repositories []githubRepoJSON `json:"repositories"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil
	}
	return mapGitHubRepos(result.Repositories), nil
}

func (p *githubProvider) listReposFallbackPage(token, org string, page int) ([]GitRepo, error) {
	var apiURL string
	if org != "" {
		apiURL = fmt.Sprintf(
			"https://api.github.com/user/repos?per_page=%d&page=%d&sort=updated&affiliation=organization_member",
			githubSearchPerPage, page,
		)
	} else {
		apiURL = fmt.Sprintf(
			"https://api.github.com/user/repos?per_page=%d&page=%d&sort=updated&affiliation=owner,collaborator",
			githubSearchPerPage, page,
		)
	}
	status, body, err := p.getJSON(apiURL, token)
	if err != nil {
		return nil, err
	}
	if status == 401 || status == 403 {
		return nil, fmt.Errorf("GitHub token expired or revoked (HTTP %d) — please reconnect in Resources", status)
	}

	var repos []githubRepoJSON
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, nil
	}
	var out []GitRepo
	for _, r := range repos {
		if org != "" && !strings.HasPrefix(r.FullName, org+"/") {
			continue
		}
		out = append(out, GitRepo{
			Name:          r.Name,
			FullName:      r.FullName,
			CloneURL:      r.CloneURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return out, nil
}

func (p *githubProvider) searchReposAPI(token, org, username, query string) ([]GitRepo, error) {
	qualifier := ""
	if org != "" {
		qualifier = "org:" + org
	} else if username != "" {
		qualifier = "user:" + username
	}
	searchQ := strings.TrimSpace(query)
	if qualifier != "" {
		searchQ = qualifier + " " + searchQ
	}
	searchQ += " in:name fork:true"

	params := url.Values{}
	params.Set("per_page", strconv.Itoa(githubSearchPerPage))
	params.Set("q", searchQ)
	apiURL := "https://api.github.com/search/repositories?" + params.Encode()

	status, body, err := p.getJSON(apiURL, token)
	if err != nil {
		return nil, err
	}
	if status == 401 || status == 403 {
		return nil, fmt.Errorf("GitHub token expired or revoked (HTTP %d) — please reconnect in Resources", status)
	}

	var result struct {
		Items []githubRepoJSON `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil
	}
	return mapGitHubRepos(result.Items), nil
}

func mapGitHubRepos(repos []githubRepoJSON) []GitRepo {
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

// getJSON performs an authenticated GET and returns the status and body,
// closing the response body before returning.
func (p *githubProvider) getJSON(url, token string) (int, []byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	return resp.StatusCode, body, nil
}

// listRepos prefers the GitHub App installation repositories endpoint (only the
// repos the user authorized) and falls back to /user/repos for classic OAuth.
func (p *githubProvider) listRepos(token, org string) ([]GitRepo, error) {
	installationID, err := p.findInstallation(token, org)
	if err != nil || installationID == 0 {
		return p.listReposFallback(token, org)
	}

	var allRepos []GitRepo
	page := 1
	for {
		apiURL := fmt.Sprintf("https://api.github.com/user/installations/%d/repositories?per_page=100&page=%d", installationID, page)
		status, body, err := p.getJSON(apiURL, token)
		if err != nil {
			return nil, err
		}
		if status == 401 || status == 403 {
			return nil, fmt.Errorf("GitHub token expired or revoked (HTTP %d) — please reconnect in Resources", status)
		}

		var result struct {
			Repositories []struct {
				Name          string `json:"name"`
				FullName      string `json:"full_name"`
				CloneURL      string `json:"clone_url"`
				DefaultBranch string `json:"default_branch"`
				Private       bool   `json:"private"`
			} `json:"repositories"`
			TotalCount int `json:"total_count"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return allRepos, nil
		}
		for _, r := range result.Repositories {
			allRepos = append(allRepos, GitRepo{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
			})
		}
		if len(result.Repositories) < 100 {
			break
		}
		page++
		if page > 10 {
			break
		}
	}
	return allRepos, nil
}

// findInstallation queries /user/installations to find the App installation ID.
// If org is set, it looks for the installation matching that org account.
func (p *githubProvider) findInstallation(token, org string) (int64, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/installations?per_page=100", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Installations []struct {
			ID      int64 `json:"id"`
			Account struct {
				Login string `json:"login"`
			} `json:"account"`
		} `json:"installations"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	for _, inst := range result.Installations {
		if org != "" && strings.EqualFold(inst.Account.Login, org) {
			return inst.ID, nil
		}
		if org == "" {
			return inst.ID, nil
		}
	}
	return 0, nil
}

// listReposFallback is used when no App installation is found (classic OAuth).
func (p *githubProvider) listReposFallback(token, org string) ([]GitRepo, error) {
	var allRepos []GitRepo
	page := 1
	for {
		var apiURL string
		if org != "" {
			apiURL = fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d&sort=updated&affiliation=organization_member", page)
		} else {
			apiURL = fmt.Sprintf("https://api.github.com/user/repos?per_page=100&page=%d&sort=updated&affiliation=owner,collaborator", page)
		}
		status, body, err := p.getJSON(apiURL, token)
		if err != nil {
			return nil, err
		}
		if status == 401 || status == 403 {
			return nil, fmt.Errorf("GitHub token expired or revoked (HTTP %d) — please reconnect in Resources", status)
		}

		var repos []struct {
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			CloneURL      string `json:"clone_url"`
			DefaultBranch string `json:"default_branch"`
			Private       bool   `json:"private"`
		}
		if err := json.Unmarshal(body, &repos); err != nil {
			return allRepos, nil
		}
		for _, r := range repos {
			if org != "" && !strings.HasPrefix(r.FullName, org+"/") {
				continue
			}
			allRepos = append(allRepos, GitRepo{
				Name:          r.Name,
				FullName:      r.FullName,
				CloneURL:      r.CloneURL,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
			})
		}
		if len(repos) < 100 {
			break
		}
		page++
		if page > 10 {
			break
		}
	}
	return allRepos, nil
}

// CloneToken mints a fresh GitHub App installation access token for the repo's
// owner. It falls back to the stored OAuth token when the App is not configured.
func (p *githubProvider) CloneToken(ctx context.Context, repoURL string, cfg json.RawMessage) (string, error) {
	token, err := p.installationToken(ctx, repoURL)
	if err == nil && token != "" {
		return token, nil
	}
	var c githubConfig
	if len(cfg) > 0 && json.Unmarshal(cfg, &c) == nil && c.Token != "" {
		return c.Token, nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("no github token available")
}

// installationToken creates a fresh GitHub App installation access token.
func (p *githubProvider) installationToken(ctx context.Context, repoURL string) (string, error) {
	appIDStr, err := p.settings.Get(ctx, "github_app_id")
	if err != nil || appIDStr == "" {
		return "", fmt.Errorf("github_app_id not configured")
	}
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid github_app_id: %w", err)
	}

	pemKey, err := p.settings.Get(ctx, "github_app_pem")
	if err != nil || pemKey == "" {
		return "", fmt.Errorf("github_app_pem not configured")
	}

	jwtToken, err := generateAppJWT(appID, pemKey)
	if err != nil {
		return "", fmt.Errorf("generate JWT: %w", err)
	}

	owner := extractGitHubOwner(repoURL)
	if owner == "" {
		return "", fmt.Errorf("cannot extract owner from %s", repoURL)
	}

	installationID, err := p.findAppInstallation(ctx, jwtToken, owner)
	if err != nil {
		return "", fmt.Errorf("find installation: %w", err)
	}

	token, err := p.createInstallationToken(ctx, jwtToken, installationID)
	if err != nil {
		return "", fmt.Errorf("create installation token: %w", err)
	}
	return token, nil
}

// generateAppJWT creates a short-lived JWT signed with the GitHub App's private key.
func generateAppJWT(appID int64, pemKeyStr string) (string, error) {
	block, _ := pem.Decode([]byte(pemKeyStr))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(appID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(key)
}

// extractGitHubOwner gets the org/user from a GitHub URL.
func extractGitHubOwner(repoURL string) string {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	for i, part := range parts {
		if part == "github.com" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// findAppInstallation finds the GitHub App installation ID for an owner.
func (p *githubProvider) findAppInstallation(ctx context.Context, jwtToken, owner string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/app/installations", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("list installations: %s %s", resp.Status, string(body))
	}

	var installations []struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &installations); err != nil {
		return 0, fmt.Errorf("parse installations: %w", err)
	}

	for _, inst := range installations {
		if strings.EqualFold(inst.Account.Login, owner) {
			return inst.ID, nil
		}
	}
	return 0, fmt.Errorf("no installation found for %s", owner)
}

// createInstallationToken creates a fresh access token for an installation.
func (p *githubProvider) createInstallationToken(ctx context.Context, jwtToken string, installationID int64) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return "", fmt.Errorf("create token: %s %s", resp.Status, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	return result.Token, nil
}

// VerifyWebhook validates a GitHub webhook using the X-Hub-Signature-256 HMAC.
func (p *githubProvider) VerifyWebhook(secret string, body []byte, headers http.Header) bool {
	signature := headers.Get("X-Hub-Signature-256")
	sig := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func init() {
	registerGit(func(d factoryDeps) GitProvider {
		return newGitHubProvider(d.settings, d.client, d.logger)
	})
}
