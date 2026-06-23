package cloudflare

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
)

// PagesProject is a Cloudflare Pages project.
type PagesProject struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Subdomain       string `json:"subdomain"`
	ProductionURL   string `json:"production_url"`
	CanonicalDeploy string `json:"canonical_deployment"`
}

// PagesDeployment is a Cloudflare Pages deployment.
type PagesDeployment struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Stage   string `json:"stage"`
	Env     string `json:"environment"`
	ShortID string `json:"short_id"`
}

// PagesDomain is a custom domain attached to a Pages project.
type PagesDomain struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListPagesProjects returns Pages projects in the account.
func (c *Client) ListPagesProjects(ctx context.Context, creds Credentials) ([]PagesProject, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	accountID, err := creds.requireAccountID()
	if err != nil {
		return nil, err
	}
	var projects []PagesProject
	page := 1
	for {
		path := fmt.Sprintf("/accounts/%s/pages/projects?per_page=50&page=%d", accountID, page)
		var batch []PagesProject
		if err := c.do(ctx, creds, http.MethodGet, path, nil, &batch); err != nil {
			return nil, err
		}
		projects = append(projects, batch...)
		if len(batch) < 50 {
			break
		}
		page++
	}
	return projects, nil
}

// CreatePagesProject creates a Direct Upload Pages project.
func (c *Client) CreatePagesProject(ctx context.Context, creds Credentials, name string) (*PagesProject, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	accountID, err := creds.requireAccountID()
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	body := map[string]any{
		"name":                           name,
		"production_branch":              "main",
		"production_deployments_enabled": false,
	}
	var out PagesProject
	path := fmt.Sprintf("/accounts/%s/pages/projects", accountID)
	if err := c.do(ctx, creds, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPagesProject returns a Pages project by name.
func (c *Client) GetPagesProject(ctx context.Context, creds Credentials, projectName string) (*PagesProject, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	accountID, err := creds.requireAccountID()
	if err != nil {
		return nil, err
	}
	var out PagesProject
	path := fmt.Sprintf("/accounts/%s/pages/projects/%s", accountID, urlPathEscape(projectName))
	if err := c.do(ctx, creds, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeletePagesProject removes a Pages project.
func (c *Client) DeletePagesProject(ctx context.Context, creds Credentials, projectName string) error {
	if err := creds.Validate(); err != nil {
		return err
	}
	accountID, err := creds.requireAccountID()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/accounts/%s/pages/projects/%s", accountID, urlPathEscape(projectName))
	return c.do(ctx, creds, http.MethodDelete, path, nil, nil)
}

// PagesUploadToken returns a JWT for Direct Upload asset uploads.
func (c *Client) PagesUploadToken(ctx context.Context, creds Credentials, projectName string) (string, error) {
	if err := creds.Validate(); err != nil {
		return "", err
	}
	accountID, err := creds.requireAccountID()
	if err != nil {
		return "", err
	}
	var out struct {
		JWT string `json:"jwt"`
	}
	path := fmt.Sprintf("/accounts/%s/pages/projects/%s/upload-token", accountID, urlPathEscape(projectName))
	if err := c.do(ctx, creds, http.MethodGet, path, nil, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.JWT) == "" {
		return "", fmt.Errorf("empty upload token from Cloudflare")
	}
	return out.JWT, nil
}

// UploadPagesAsset uploads a single file blob keyed by its content hash.
func (c *Client) UploadPagesAsset(ctx context.Context, jwt string, hash string, content []byte) error {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return fmt.Errorf("asset hash is required")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("CF-UPLOAD-JWT", jwt); err != nil {
		return err
	}
	part, err := w.CreateFormFile(hash, path.Base(hash))
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	base := c.baseURL
	if base == "" {
		base = baseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/pages/assets/upload", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var envelope apiResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("parse upload response: %w", err)
	}
	if !envelope.Success {
		if len(envelope.Errors) > 0 {
			return fmt.Errorf("%s", envelope.Errors[0].Message)
		}
		return fmt.Errorf("cloudflare asset upload failed")
	}
	return nil
}

// CreatePagesDeployment creates a deployment from a path→hash manifest.
func (c *Client) CreatePagesDeployment(ctx context.Context, creds Credentials, projectName string, manifest map[string]string) (*PagesDeployment, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	accountID, err := creds.requireAccountID()
	if err != nil {
		return nil, err
	}
	if len(manifest) == 0 {
		return nil, fmt.Errorf("deployment manifest is empty")
	}
	body := map[string]any{"manifest": manifest}
	var out PagesDeployment
	path := fmt.Sprintf("/accounts/%s/pages/projects/%s/deployments", accountID, urlPathEscape(projectName))
	if err := c.do(ctx, creds, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddPagesDomain attaches a custom domain to a Pages project.
func (c *Client) AddPagesDomain(ctx context.Context, creds Credentials, projectName, domain string) (*PagesDomain, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	accountID, err := creds.requireAccountID()
	if err != nil {
		return nil, err
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	body := map[string]any{"name": domain}
	var out PagesDomain
	path := fmt.Sprintf("/accounts/%s/pages/projects/%s/domains", accountID, urlPathEscape(projectName))
	if err := c.do(ctx, creds, http.MethodPost, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PagesDefaultURL builds the default *.pages.dev URL for a project.
func PagesDefaultURL(subdomain string) string {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return ""
	}
	return "https://" + subdomain + ".pages.dev"
}

// PagesAssetHash returns the SHA-256 hex digest used as the Direct Upload key.
func PagesAssetHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func (c Credentials) requireAccountID() (string, error) {
	id := strings.TrimSpace(c.AccountID)
	if id == "" {
		return "", fmt.Errorf("account_id is required on the Cloudflare cloud account")
	}
	return id, nil
}

func urlPathEscape(segment string) string {
	return strings.NewReplacer("/", "%2F").Replace(segment)
}
