package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// pageTeardownTimeout bounds a detached best-effort cloud teardown so a stuck
// AWS call can't leak a goroutine indefinitely.
const pageTeardownTimeout = 10 * time.Minute

type PageService struct {
	store    store.Store
	registry *pages.Registry
	logger   *slog.Logger
	notifSvc *NotificationService

	// teardownWG tracks in-flight detached cloud teardowns so they can be
	// awaited (graceful shutdown, tests).
	teardownWG sync.WaitGroup
}

func NewPageService(s store.Store, registry *pages.Registry, logger *slog.Logger, notifSvc *NotificationService) *PageService {
	return &PageService{store: s, registry: registry, logger: logger, notifSvc: notifSvc}
}

// WaitForTeardowns blocks until all detached cloud teardowns started by Delete
// have finished. Intended for graceful shutdown and deterministic tests.
func (s *PageService) WaitForTeardowns() { s.teardownWG.Wait() }

type CreatePageInput struct {
	ProjectID      uuid.UUID          `json:"project_id" binding:"required"`
	Name           string             `json:"name" binding:"required,min=1,max=63"`
	Description    string             `json:"description"`
	GitRepo        string             `json:"git_repo" binding:"required"`
	GitBranch      string             `json:"git_branch"`
	GitProviderID  *uuid.UUID         `json:"git_provider_id"`
	PublishPath    string             `json:"publish_path"`
	Provider       model.PageProvider `json:"provider"`
	CloudAccountID *uuid.UUID         `json:"cloud_account_id"`
	Region         string             `json:"region"`
	CustomDomain   string             `json:"custom_domain"`
	ManageDNS      bool               `json:"manage_dns"`
	DNSAccountID   *uuid.UUID         `json:"dns_account_id"`
	DNSZoneID      string             `json:"dns_zone_id"`

	// Build configuration (optional).
	BuildEnabled   bool              `json:"build_enabled"`
	PackageManager string            `json:"package_manager"`
	InstallCommand string            `json:"install_command"`
	BuildCommand   string            `json:"build_command"`
	OutputDir      string            `json:"output_dir"`
	RootDirectory  string            `json:"root_directory"`
	NodeVersion    string            `json:"node_version"`
	BuildEnvVars   map[string]string `json:"build_env_vars"`
}

func (s *PageService) Create(ctx context.Context, input CreatePageInput) (*model.Page, error) {
	if !safeNameRe.MatchString(input.Name) {
		return nil, fmt.Errorf("page name must start with alphanumeric and contain only letters, numbers, hyphens, and underscores")
	}

	project, err := s.store.Projects().GetByID(ctx, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	if input.CloudAccountID != nil {
		if err := s.validateResource(ctx, project.OrgID, *input.CloudAccountID, model.ResourceCloudAccount); err != nil {
			return nil, err
		}
		if err := validatePageCloudAccount(ctx, s.store, input.Provider, *input.CloudAccountID); err != nil {
			return nil, err
		}
	}
	if input.GitProviderID != nil {
		if err := s.validateResource(ctx, project.OrgID, *input.GitProviderID, model.ResourceGitProvider); err != nil {
			return nil, err
		}
	}

	provider := input.Provider
	if provider == "" {
		provider = model.PageProviderAWSCloudFront
	}
	if provider != model.PageProviderAWSCloudFront && provider != model.PageProviderCloudflarePages {
		return nil, fmt.Errorf("provider must be aws_cloudfront or cloudflare_pages")
	}

	customDomain := normalizeDomain(input.CustomDomain)
	if customDomain != "" {
		if err := s.validateCustomDomain(ctx, project.OrgID, provider, customDomain, input.ManageDNS, input.DNSAccountID, input.DNSZoneID); err != nil {
			return nil, err
		}
	} else if input.ManageDNS {
		return nil, fmt.Errorf("custom_domain is required when manage_dns is enabled")
	}

	if err := validatePackageManager(input.PackageManager); err != nil {
		return nil, err
	}

	buildEnv := input.BuildEnvVars
	if buildEnv == nil {
		buildEnv = map[string]string{}
	}

	page := &model.Page{
		ProjectID:      input.ProjectID,
		Name:           input.Name,
		Description:    input.Description,
		GitRepo:        input.GitRepo,
		GitBranch:      input.GitBranch,
		GitProviderID:  input.GitProviderID,
		PublishPath:    input.PublishPath,
		BuildEnabled:   input.BuildEnabled,
		PackageManager: input.PackageManager,
		InstallCommand: input.InstallCommand,
		BuildCommand:   input.BuildCommand,
		OutputDir:      strings.TrimSpace(input.OutputDir),
		RootDirectory:  input.RootDirectory,
		NodeVersion:    strings.TrimSpace(input.NodeVersion),
		BuildEnvVars:   buildEnv,
		Provider:       provider,
		CloudAccountID: input.CloudAccountID,
		Region:         input.Region,
		CustomDomain:   customDomain,
		ManageDNS:      input.ManageDNS,
		DNSAccountID:   input.DNSAccountID,
		DNSZoneID:      strings.TrimSpace(input.DNSZoneID),
		Runtime:        &model.PageRuntime{},
		Status:         model.PageStatusIdle,
	}
	if page.GitBranch == "" {
		page.GitBranch = "main"
	}
	if page.PublishPath == "" {
		page.PublishPath = "."
	}
	if page.PackageManager == "" {
		page.PackageManager = "auto"
	}
	if page.RootDirectory == "" {
		page.RootDirectory = "."
	}
	if page.BuildEnabled && page.OutputDir == "" {
		page.OutputDir = "dist"
	}
	if page.Region == "" && page.Provider == model.PageProviderAWSCloudFront {
		page.Region = "us-east-1"
	}

	if dup, err := s.store.Pages().ExistsByName(ctx, input.ProjectID, page.Name); err != nil {
		return nil, fmt.Errorf("cannot verify name uniqueness: %w", err)
	} else if dup {
		return nil, fmt.Errorf("a page named %q already exists in this project", page.Name)
	}

	if err := s.store.Pages().Create(ctx, page); err != nil {
		// Partial index (post-migration) or legacy constraint name on older DBs.
		if strings.Contains(err.Error(), "idx_pages_project_id_name_active") ||
			strings.Contains(err.Error(), "pages_project_id_name_key") {
			return nil, fmt.Errorf("a page named %q already exists in this project", page.Name)
		}
		return nil, err
	}
	s.logger.Info("page created", slog.String("name", page.Name), slog.String("id", page.ID.String()))
	return page, nil
}

func (s *PageService) GetByID(ctx context.Context, id uuid.UUID) (*model.Page, error) {
	return s.store.Pages().GetByID(ctx, id)
}

// validatePackageManager rejects an unsupported package manager value. Empty is
// allowed (defaulted to "auto" elsewhere).
func validatePackageManager(packageManager string) error {
	switch strings.TrimSpace(packageManager) {
	case "", "auto", "npm", "pnpm":
		return nil
	default:
		return fmt.Errorf("package_manager must be one of: auto, npm, pnpm")
	}
}

func (s *PageService) List(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Page, int, error) {
	return s.store.Pages().ListByProject(ctx, projectID, params)
}

func (s *PageService) ListAll(ctx context.Context, params store.ListParams, filter store.PageListFilter) ([]model.Page, int, error) {
	return s.store.Pages().ListAll(ctx, params, filter)
}

type UpdatePageInput struct {
	Description    *string    `json:"description"`
	GitRepo        *string    `json:"git_repo"`
	GitBranch      *string    `json:"git_branch"`
	GitProviderID  *uuid.UUID `json:"git_provider_id"`
	PublishPath    *string    `json:"publish_path"`
	CloudAccountID *uuid.UUID `json:"cloud_account_id"`
	Region         *string    `json:"region"`
	CustomDomain   *string    `json:"custom_domain"`
	ManageDNS      *bool      `json:"manage_dns"`
	DNSAccountID   *uuid.UUID `json:"dns_account_id"`
	DNSZoneID      *string    `json:"dns_zone_id"`

	// Build configuration (optional).
	BuildEnabled   *bool              `json:"build_enabled"`
	PackageManager *string            `json:"package_manager"`
	InstallCommand *string            `json:"install_command"`
	BuildCommand   *string            `json:"build_command"`
	OutputDir      *string            `json:"output_dir"`
	RootDirectory  *string            `json:"root_directory"`
	NodeVersion    *string            `json:"node_version"`
	BuildEnvVars   *map[string]string `json:"build_env_vars"`
}

func (s *PageService) Update(ctx context.Context, id uuid.UUID, input UpdatePageInput) (*model.Page, error) {
	page, err := s.store.Pages().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Description != nil {
		page.Description = *input.Description
	}
	if input.GitRepo != nil {
		page.GitRepo = *input.GitRepo
	}
	if input.GitBranch != nil {
		page.GitBranch = *input.GitBranch
	}
	if input.BuildEnabled != nil {
		page.BuildEnabled = *input.BuildEnabled
	}
	if input.PackageManager != nil {
		if err := validatePackageManager(*input.PackageManager); err != nil {
			return nil, err
		}
		pm := strings.TrimSpace(*input.PackageManager)
		if pm == "" {
			pm = "auto"
		}
		page.PackageManager = pm
	}
	if input.InstallCommand != nil {
		page.InstallCommand = *input.InstallCommand
	}
	if input.BuildCommand != nil {
		page.BuildCommand = *input.BuildCommand
	}
	if input.OutputDir != nil {
		page.OutputDir = strings.TrimSpace(*input.OutputDir)
	}
	if input.RootDirectory != nil {
		root := strings.TrimSpace(*input.RootDirectory)
		if root == "" {
			root = "."
		}
		page.RootDirectory = root
	}
	if input.NodeVersion != nil {
		page.NodeVersion = strings.TrimSpace(*input.NodeVersion)
	}
	if input.BuildEnvVars != nil {
		page.BuildEnvVars = *input.BuildEnvVars
	}
	// Default output dir when build is enabled but none was ever set.
	if page.BuildEnabled && strings.TrimSpace(page.OutputDir) == "" {
		page.OutputDir = "dist"
	}
	// Validate referenced shared resources against the project's org before
	// rebinding them, so a page can't be pointed at another org's
	// credentials (which a later deploy would then use).
	var orgID *uuid.UUID
	resolveOrg := func() (uuid.UUID, error) {
		if orgID != nil {
			return *orgID, nil
		}
		project, perr := s.store.Projects().GetByID(ctx, page.ProjectID)
		if perr != nil {
			return uuid.Nil, fmt.Errorf("project not found: %w", perr)
		}
		orgID = &project.OrgID
		return project.OrgID, nil
	}
	if input.GitProviderID != nil {
		org, oerr := resolveOrg()
		if oerr != nil {
			return nil, oerr
		}
		if verr := s.validateResource(ctx, org, *input.GitProviderID, model.ResourceGitProvider); verr != nil {
			return nil, verr
		}
		page.GitProviderID = input.GitProviderID
	}
	if input.PublishPath != nil {
		page.PublishPath = *input.PublishPath
	}
	// Lock domain editing once EITHER a cert has been requested OR the CloudFront
	// distribution already exists. The distribution's Aliases + ViewerCertificate
	// are only set at creation time (Provision skips createDistribution when a
	// distribution already exists), so adding/changing the custom domain after
	// the distribution is live can never attach the alias — every request to the
	// custom domain would 421. Changing the domain on an existing distribution is
	// out of scope (would require UpdateDistribution).
	domainLocked := pageDomainLocked(page)
	if !domainLocked {
		if input.CustomDomain != nil {
			page.CustomDomain = normalizeDomain(*input.CustomDomain)
		}
		if input.ManageDNS != nil {
			page.ManageDNS = *input.ManageDNS
		}
		if input.DNSAccountID != nil {
			page.DNSAccountID = input.DNSAccountID
		}
		if input.DNSZoneID != nil {
			page.DNSZoneID = strings.TrimSpace(*input.DNSZoneID)
		}
		org, oerr := resolveOrg()
		if oerr != nil {
			return nil, oerr
		}
		// Only re-validate the domain when this PATCH actually touches a domain
		// field; otherwise an unrelated save (e.g. publish_path) would re-run the
		// zone check and could fail on DNS that changed for unrelated reasons.
		domainFieldsChanged := input.CustomDomain != nil || input.ManageDNS != nil ||
			input.DNSAccountID != nil || input.DNSZoneID != nil
		if domainFieldsChanged && page.CustomDomain != "" {
			if err := s.validateCustomDomain(ctx, org, page.Provider, page.CustomDomain, page.ManageDNS, page.DNSAccountID, page.DNSZoneID); err != nil {
				return nil, err
			}
		} else if page.ManageDNS && page.CustomDomain == "" {
			return nil, fmt.Errorf("custom_domain is required when manage_dns is enabled")
		}
	} else if input.CustomDomain != nil || input.ManageDNS != nil || input.DNSAccountID != nil || input.DNSZoneID != nil {
		return nil, fmt.Errorf("cannot change custom domain settings after the page has been provisioned")
	}
	// cloudTargetChanging records whether this PATCH repoints the page's cloud
	// account or region. Those changes must not land while a deploy is
	// provisioning into the current target, so they are persisted with an
	// atomic "status <> deploying" guard below.
	cloudTargetChanging := false
	if input.CloudAccountID != nil {
		// Changing the cloud account after provisioning would point deploys at a
		// different account's credentials while the bucket/distribution still live
		// in the original account, failing every deploy with AccessDenied; only
		// allow it before the first deploy. The status check closes the window
		// between TryMarkDeploying and the first runtime save where Runtime is
		// still empty but the worker is already provisioning into the current
		// account — a PATCH there would silently retarget mid-provision.
		changing := page.CloudAccountID == nil || *input.CloudAccountID != *page.CloudAccountID
		if changing && (page.Status == model.PageStatusDeploying || pageCloudProvisioned(page)) {
			return nil, fmt.Errorf("cannot change cloud account while a deploy is in progress or after the first deploy")
		}
		org, oerr := resolveOrg()
		if oerr != nil {
			return nil, oerr
		}
		if verr := s.validateResource(ctx, org, *input.CloudAccountID, model.ResourceCloudAccount); verr != nil {
			return nil, verr
		}
		if verr := validatePageCloudAccount(ctx, s.store, page.Provider, *input.CloudAccountID); verr != nil {
			return nil, verr
		}
		page.CloudAccountID = input.CloudAccountID
		cloudTargetChanging = cloudTargetChanging || changing
	}
	if input.Region != nil {
		// Changing region after provisioning would orphan the existing bucket;
		// only allow it before the first deploy. As with cloud_account_id, the
		// status check closes the provisioning window where Runtime is not yet
		// populated but a deploy is already creating resources in page.Region.
		if page.Provider != model.PageProviderAWSCloudFront {
			return nil, fmt.Errorf("region can only be set for AWS CloudFront pages")
		}
		changing := *input.Region != page.Region
		if changing && (page.Status == model.PageStatusDeploying || (page.Runtime != nil && page.Runtime.BucketName != "")) {
			return nil, fmt.Errorf("cannot change region while a deploy is in progress or after the first deploy")
		}
		page.Region = *input.Region
		cloudTargetChanging = cloudTargetChanging || changing
	}
	// Persist only the editable settings columns. A deploy may be running
	// concurrently (CloudFront provisioning takes ~10 minutes), and the worker
	// writes runtime IDs (BucketName/DistributionID/OACID) and status
	// incrementally. A full-row save here would overwrite that runtime state
	// with the snapshot loaded at the top of this function, orphaning the
	// already-provisioned AWS resources on the next deploy.
	//
	// When the cloud target changes, persist with an atomic "status <>
	// deploying" guard: the in-memory check above acts on a GetByID snapshot, so
	// without it a deploy that started between the read and this write would be
	// left provisioning into the old account/region while the row points at the
	// new one — a permanently undeployable page.
	if cloudTargetChanging {
		applied, err := s.store.Pages().UpdateSettingsIfNotDeploying(ctx, page)
		if err != nil {
			return nil, err
		}
		if !applied {
			return nil, fmt.Errorf("cannot change cloud account or region while a deploy is in progress")
		}
		return page, nil
	}
	if err := s.store.Pages().UpdateSettings(ctx, page); err != nil {
		return nil, err
	}
	return page, nil
}

func (s *PageService) Delete(ctx context.Context, id uuid.UUID) error {
	// Fast, friendly pre-check that also preserves the not-found path: surface a
	// clear "deploying" message instead of relying solely on the atomic guard
	// (which can't distinguish "deploying" from "already gone").
	page, err := s.store.Pages().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if page.Status == model.PageStatusDeploying {
		return fmt.Errorf("page is currently deploying, wait for it to finish before deleting")
	}

	// Authoritative atomic guard. Remove the row only if it isn't mid-deploy,
	// and do it BEFORE any cloud teardown. This closes the TOCTOU window with
	// TryMarkDeploying: the pre-check above reads a snapshot, so a deploy could
	// start between it and here. Deleting first means once the row is gone no
	// deploy can start (the worker's TryMarkDeploying/UpdateRuntime match 0
	// rows), and if a deploy did slip in we match 0 rows and refuse without
	// having touched any AWS resources — so we never tear down a bucket out from
	// under an in-flight deploy.
	deleted, err := s.store.Pages().DeleteIfNotDeploying(ctx, id)
	if err != nil {
		return err
	}
	if deleted == nil {
		return fmt.Errorf("page is currently deploying, wait for it to finish before deleting")
	}

	// Best-effort cloud teardown so test accounts don't accumulate orphaned
	// buckets/distributions before the full delete flow lands in Phase 4. The
	// DB row is already gone, so we detach teardown from the HTTP request: it
	// makes potentially slow S3/CloudFront calls (emptying a large bucket isn't
	// instant), and running it inline would block the response and — worse —
	// let a client disconnect cancel ctx mid-teardown, orphaning whatever wasn't
	// deleted yet. (Phase 4 replaces this with a proper async teardown job +
	// `draining` status.)
	if deleted.Runtime != nil && pageCloudProvisioned(deleted) {
		s.teardownCloudAsync(ctx, deleted)
	}

	notifyProjectResourceDeleted(s.notifSvc, s.store, s.logger, deleted.ProjectID, model.EventPageDeleted,
		deleted.Name, fmt.Sprintf("Page %q was deleted", deleted.Name))
	return nil
}

// teardownCloudAsync runs the best-effort cloud teardown in the background,
// detached from the caller's context lifecycle (so a finished/cancelled HTTP
// request can't abort it) but bounded by pageTeardownTimeout. Failures are
// logged for manual follow-up; the DB record is already removed.
func (s *PageService) teardownCloudAsync(ctx context.Context, page *model.Page) {
	// Preserve request-scoped values (logging/tracing) but drop cancellation.
	bg := context.WithoutCancel(ctx)
	s.teardownWG.Add(1)
	go func() {
		defer s.teardownWG.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("page delete: cloud teardown panicked — clean up AWS manually",
					slog.String("page", page.Name), slog.Any("panic", r))
			}
		}()

		tctx, cancel := context.WithTimeout(bg, pageTeardownTimeout)
		defer cancel()

		provider, cfg, perr := s.resolveProvider(tctx, page)
		if perr != nil {
			s.logger.Warn("page delete: cannot resolve provider for cloud teardown — DB record already removed",
				slog.String("page", page.Name), slog.Any("error", perr))
			return
		}
		if derr := provider.Delete(tctx, page, cfg); derr != nil {
			s.logger.Warn("page delete: cloud teardown failed — DB record removed, clean up AWS manually",
				slog.String("page", page.Name),
				slog.String("bucket", page.Runtime.BucketName),
				slog.String("distribution", page.Runtime.DistributionID),
				slog.Any("error", derr))
		}
	}()
}

// ListDeployments returns the deployment history for a page.
func (s *PageService) ListDeployments(ctx context.Context, pageID uuid.UUID, params store.ListParams) ([]model.PageDeployment, int, error) {
	return s.store.PageDeployments().ListByPage(ctx, pageID, params)
}

func (s *PageService) GetDeployment(ctx context.Context, id uuid.UUID) (*model.PageDeployment, error) {
	return s.store.PageDeployments().GetByID(ctx, id)
}

// validateResource ensures a referenced shared resource exists, has the expected
// type, and belongs to the given org.
func (s *PageService) validateResource(ctx context.Context, orgID, id uuid.UUID, want model.ResourceType) error {
	res, err := s.store.SharedResources().GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("%s resource not found: %w", want, err)
	}
	if res.Type != want {
		return fmt.Errorf("resource %s is not a %s", res.Name, want)
	}
	if res.OrgID != orgID {
		return fmt.Errorf("resource %s does not belong to this organization", res.Name)
	}
	return nil
}

// resolveProvider returns the configured PagesProvider plus raw cloud account
// config for the page. Shared by the deploy and delete paths.
func (s *PageService) resolveProvider(ctx context.Context, page *model.Page) (pages.PagesProvider, json.RawMessage, error) {
	provider, err := s.registry.Get(string(page.Provider))
	if err != nil {
		return nil, nil, err
	}
	cfg, err := resolveCloudConfig(ctx, s.store, page.CloudAccountID)
	if err != nil {
		return nil, nil, err
	}
	return provider, cfg, nil
}

// resolveCloudConfig loads a cloud_account resource config blob.
func resolveCloudConfig(ctx context.Context, st store.Store, cloudAccountID *uuid.UUID) (json.RawMessage, error) {
	if cloudAccountID == nil {
		return nil, fmt.Errorf("no cloud account configured for this page")
	}
	res, err := st.SharedResources().GetByID(ctx, *cloudAccountID)
	if err != nil {
		return nil, fmt.Errorf("cloud account not found: %w", err)
	}
	if res.Type != model.ResourceCloudAccount {
		return nil, fmt.Errorf("resource %s is not a cloud account", res.Name)
	}
	return res.Config, nil
}

func pageCloudProvisioned(page *model.Page) bool {
	if page.Runtime == nil {
		return false
	}
	switch page.Provider {
	case model.PageProviderCloudflarePages:
		return page.Runtime.CFProjectID != ""
	default:
		return page.Runtime.BucketName != "" || page.Runtime.DistributionID != ""
	}
}

func pageDomainLocked(page *model.Page) bool {
	if page.Runtime == nil {
		return false
	}
	switch page.Provider {
	case model.PageProviderCloudflarePages:
		return page.Runtime.CFProjectID != ""
	default:
		return page.Runtime.CertificateARN != "" || page.Runtime.DistributionID != ""
	}
}

func validatePageCloudAccount(ctx context.Context, st store.Store, pageProvider model.PageProvider, cloudAccountID uuid.UUID) error {
	if pageProvider == "" {
		pageProvider = model.PageProviderAWSCloudFront
	}
	res, err := st.SharedResources().GetByID(ctx, cloudAccountID)
	if err != nil {
		return fmt.Errorf("cloud account not found: %w", err)
	}
	var want string
	switch pageProvider {
	case model.PageProviderCloudflarePages:
		want = "cloudflare"
	case model.PageProviderAWSCloudFront:
		want = "aws"
	default:
		return fmt.Errorf("unsupported page provider %q", pageProvider)
	}
	prov := res.Provider
	if prov == "" {
		prov = "aws"
	}
	if prov != want {
		return fmt.Errorf("page provider %q requires a %s cloud account, but %q was selected", pageProvider, want, prov)
	}
	return nil
}

// resolveCloudAccountTags loads the tag definitions configured on a cloud_account
// resource.
func resolveCloudAccountTags(ctx context.Context, s store.Store, cloudAccountID *uuid.UUID) ([]pages.Tag, error) {
	if cloudAccountID == nil {
		return nil, fmt.Errorf("no cloud account configured for this page")
	}
	res, err := s.SharedResources().GetByID(ctx, *cloudAccountID)
	if err != nil {
		return nil, fmt.Errorf("cloud account not found: %w", err)
	}
	if res.Type != model.ResourceCloudAccount {
		return nil, fmt.Errorf("resource %s is not a cloud account", res.Name)
	}
	return pages.ParseAccountTags(res.Config), nil
}
