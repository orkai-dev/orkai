// Package cloudflare implements pages.PagesProvider for Cloudflare Pages
// (Direct Upload). Orkai builds/clones static files and uploads them via the
// Pages Direct Upload API — no git-connected CF Pages projects.
package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	cf "github.com/orkai-dev/orkai/apps/api/internal/cloudflare"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	"golang.org/x/sync/errgroup"
)

const (
	pagesProjectPrefix   = "orkai-"
	pagesProjectMaxLen   = 50
	cfPagesUploadWorkers = 8
)

// Provider implements pages.PagesProvider for Cloudflare Pages.
type Provider struct {
	client *cf.Client
}

// New returns a Cloudflare Pages provider.
func New() *Provider { return &Provider{client: cf.NewClient()} }

// NewWithClient returns a provider using a custom API client (tests).
func NewWithClient(client *cf.Client) *Provider { return &Provider{client: client} }

// Name implements pages.PagesProvider.
func (p *Provider) Name() string { return string(model.PageProviderCloudflarePages) }

func parseCreds(cfg json.RawMessage) (cf.Credentials, error) {
	var creds cf.Credentials
	if len(cfg) == 0 {
		return creds, fmt.Errorf("empty cloud account config")
	}
	if err := json.Unmarshal(cfg, &creds); err != nil {
		return creds, fmt.Errorf("parse cloud account config: %w", err)
	}
	if err := creds.Validate(); err != nil {
		return creds, err
	}
	return creds, nil
}

// TestConnection validates credentials by listing Pages projects.
func (p *Provider) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	creds, err := parseCreds(cfg)
	if err != nil {
		return false, err.Error(), nil
	}
	projects, err := p.client.ListPagesProjects(ctx, creds)
	if err != nil {
		return false, err.Error(), nil
	}
	return true, fmt.Sprintf("Connected — %d Pages project(s) visible", len(projects)), nil
}

// Provision creates (or reuses) a Cloudflare Pages project for the Page.
func (p *Provider) Provision(ctx context.Context, page *model.Page, cfg json.RawMessage, _ map[string]string, save pages.SaveRuntime) (*model.PageRuntime, error) {
	creds, err := parseCreds(cfg)
	if err != nil {
		return nil, err
	}
	rt := page.Runtime
	if rt == nil {
		rt = &model.PageRuntime{}
	}

	projectName := rt.CFProjectName
	if projectName == "" {
		projectName = pagesProjectName(page.Name)
		rt.CFProjectName = projectName
	}

	if rt.CFProjectID == "" {
		proj, err := p.client.CreatePagesProject(ctx, creds, projectName)
		if err != nil {
			// Project may already exist from a prior partial provision.
			existing, gerr := p.client.GetPagesProject(ctx, creds, projectName)
			if gerr != nil {
				return nil, fmt.Errorf("create Pages project: %w", err)
			}
			proj = existing
		}
		rt.CFProjectID = proj.ID
		rt.CFProjectName = proj.Name
		if proj.Subdomain != "" {
			rt.DefaultURL = cf.PagesDefaultURL(proj.Subdomain)
		} else if proj.ProductionURL != "" {
			rt.DefaultURL = strings.TrimSpace(proj.ProductionURL)
		}
		if err := save(ctx, rt); err != nil {
			return nil, fmt.Errorf("persist Pages project runtime: %w", err)
		}
	}

	if rt.DefaultURL == "" && rt.CFProjectName != "" {
		rt.DefaultURL = cf.PagesDefaultURL(rt.CFProjectName)
		if err := save(ctx, rt); err != nil {
			return nil, fmt.Errorf("persist default URL: %w", err)
		}
	}

	return rt, nil
}

// Deploy uploads files via Direct Upload and creates a deployment.
func (p *Provider) Deploy(ctx context.Context, page *model.Page, cfg json.RawMessage, filesDir string, onLog func(string)) (*pages.DeployResult, error) {
	creds, err := parseCreds(cfg)
	if err != nil {
		return nil, err
	}
	if page.Runtime == nil || page.Runtime.CFProjectName == "" {
		return nil, fmt.Errorf("page not provisioned (missing Cloudflare Pages project)")
	}

	local, err := pages.CollectFiles(filesDir)
	if err != nil {
		return nil, err
	}
	onLog(fmt.Sprintf("Found %d file(s) under publish folder", len(local)))
	if len(local) == 0 {
		return nil, fmt.Errorf("publish folder is empty")
	}

	manifest := make(map[string]string, len(local))
	hashPath := make(map[string]string, len(local))
	for key, fullPath := range local {
		data, rerr := os.ReadFile(fullPath)
		if rerr != nil {
			return nil, fmt.Errorf("read %s: %w", key, rerr)
		}
		hash := cf.PagesAssetHash(data)
		manifest[key] = hash
		if _, seen := hashPath[hash]; !seen {
			hashPath[hash] = fullPath
		}
	}

	jwt, err := p.client.PagesUploadToken(ctx, creds, page.Runtime.CFProjectName)
	if err != nil {
		return nil, fmt.Errorf("request upload token: %w", err)
	}

	uploaded := 0
	var uploadedMu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfPagesUploadWorkers)
	for hash, fullPath := range hashPath {
		g.Go(func() error {
			data, rerr := os.ReadFile(fullPath)
			if rerr != nil {
				return fmt.Errorf("read %s: %w", fullPath, rerr)
			}
			if err := p.client.UploadPagesAsset(gctx, jwt, hash, data); err != nil {
				return fmt.Errorf("upload asset %s: %w", hash[:8], err)
			}
			uploadedMu.Lock()
			uploaded++
			uploadedMu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	onLog(fmt.Sprintf("Uploaded %d asset(s) to Cloudflare Pages", uploaded))

	dep, err := p.client.CreatePagesDeployment(ctx, creds, page.Runtime.CFProjectName, manifest)
	if err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}
	onLog("Cloudflare Pages deployment created: " + dep.ID)

	defaultURL := page.Runtime.DefaultURL
	if defaultURL == "" {
		defaultURL = cf.PagesDefaultURL(page.Runtime.CFProjectName)
	}

	return &pages.DeployResult{
		ProviderRef:   dep.ID,
		DefaultURL:    defaultURL,
		UploadedCount: uploaded,
	}, nil
}

// Delete removes the Cloudflare Pages project (best-effort).
func (p *Provider) Delete(ctx context.Context, page *model.Page, cfg json.RawMessage) error {
	creds, err := parseCreds(cfg)
	if err != nil {
		return err
	}
	if page.Runtime == nil || page.Runtime.CFProjectName == "" {
		return nil
	}
	if err := p.client.DeletePagesProject(ctx, creds, page.Runtime.CFProjectName); err != nil {
		return fmt.Errorf("delete Pages project: %w", err)
	}
	return nil
}

// AttachCustomDomain registers a custom domain on the Pages project.
func (p *Provider) AttachCustomDomain(ctx context.Context, cfg json.RawMessage, projectName, domain string) error {
	creds, err := parseCreds(cfg)
	if err != nil {
		return err
	}
	_, err = p.client.AddPagesDomain(ctx, creds, projectName, domain)
	return err
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9-]+`)

// pagesProjectName builds a DNS-safe Cloudflare Pages project name.
func pagesProjectName(pageName string) string {
	base := strings.ToLower(pageName)
	base = nonAlnum.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "page"
	}
	maxBase := pagesProjectMaxLen - len(pagesProjectPrefix)
	if len(base) > maxBase {
		base = strings.Trim(base[:maxBase], "-")
	}
	if base == "" {
		base = "page"
	}
	return pagesProjectPrefix + base
}
