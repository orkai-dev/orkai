package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/acm"
	"github.com/orkai-dev/orkai/apps/api/internal/dns"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

const (
	// pageBuildPhaseTimeout bounds the file-preparation phase: an in-cluster
	// build (clone + npm/pnpm install + build + output extraction) or a plain
	// git clone for pre-built pages. It is a SEPARATE budget from the deploy
	// phase below so a slow build can never starve CloudFront provisioning —
	// the original single budget could be exhausted by the build alone, making
	// provisioning fail with a misleading timeout. The in-pod build caps itself
	// (see k3s pageBuildTimeout); this adds headroom for pod scheduling, image
	// pull, and tar extraction.
	pageBuildPhaseTimeout = 40 * time.Minute
	// pageDeployPhaseTimeout bounds provisioning + sync + CDN invalidation. A
	// first deploy lazily provisions S3 + CloudFront, which can take ~10–15 min
	// to reach Deployed. This clock starts only after files are ready.
	pageDeployPhaseTimeout = 30 * time.Minute
)

// PageDeployService triggers and runs Page deployments (clone → sync → CDN
// invalidation). The first deploy of a Page also lazily provisions its AWS
// resources (S3 + CloudFront), which can take ~10 min to reach Deployed.
type PageDeployService struct {
	store      store.Store
	registry   *pages.Registry
	publishSvc *PagePublishService
	notifSvc   *NotificationService
	queue      Enqueuer
	targets    *orchestrator.TargetRegistry
	logger     *slog.Logger
}

func NewPageDeployService(
	s store.Store,
	registry *pages.Registry,
	publishSvc *PagePublishService,
	notifSvc *NotificationService,
	queue Enqueuer,
	targets *orchestrator.TargetRegistry,
	logger *slog.Logger,
) *PageDeployService {
	return &PageDeployService{
		store:      s,
		registry:   registry,
		publishSvc: publishSvc,
		notifSvc:   notifSvc,
		queue:      queue,
		targets:    targets,
		logger:     logger,
	}
}

// StartStaleRecovery marks Page deployments stuck "deploying" for >40 min as
// failed, on startup and every 5 minutes. When isLeader is non-nil, only the
// elected leader runs recovery.
func (s *PageDeployService) StartStaleRecovery(ctx context.Context, isLeader func() bool) {
	go func() {
		if shouldRunSingleton(isLeader) {
			s.recoverStale()
		}
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if shouldRunSingleton(isLeader) {
					s.recoverStale()
				}
			}
		}
	}()
}

func (s *PageDeployService) recoverStale() {
	ctx := context.Background()
	// 40 min covers first-time CloudFront creation (see CloudFront latency note).
	threshold := 40 * time.Minute
	deps, _, err := s.store.PageDeployments().ListByStatus(ctx, model.PageDeployDeploying, store.ListParams{Page: 1, PerPage: 1000})
	if err != nil {
		return
	}
	for i := range deps {
		d := deps[i]
		ref := d.CreatedAt
		if d.StartedAt != nil {
			ref = *d.StartedAt
		}
		if time.Since(ref) < threshold {
			continue
		}
		// Atomically flip to failed only if it's still deploying. The
		// ListByStatus read above is a snapshot; the worker may have finished
		// this deploy (success) in the meantime, and a full-row Update here
		// would overwrite that result. If the conditional update touches no row,
		// the deploy already reached a terminal state — leave it (and the page)
		// alone.
		marked, err := s.store.PageDeployments().MarkTimedOut(ctx, d.ID, time.Now(), "\n\n--- Deploy timed out (stale for >40min) ---")
		if err != nil || !marked {
			continue
		}
		// Move the page out of "deploying" with a write that needs no prior
		// read. MarkTimedOut already made the deployment terminal, so it won't
		// appear in a future ListByStatus scan; if we instead did GetByID +
		// setPageStatus and the GetByID failed transiently, the page would be
		// stuck "deploying" forever — and because TryMarkDeploying requires
		// status <> deploying, every future deploy would be permanently blocked.
		if uerr := s.store.Pages().UpdateStatus(ctx, d.PageID, model.PageStatusError); uerr != nil {
			s.logger.Error("stale recovery: failed to mark page errored",
				slog.String("page_id", d.PageID.String()), slog.Any("error", uerr))
		}
		s.logger.Warn("recovered stale page deployment", slog.String("deploy_id", d.ID.String()))
	}
}

// Trigger creates a queued deployment and enqueues the job.
func (s *PageDeployService) Trigger(ctx context.Context, pageID uuid.UUID, triggerType string) (*model.PageDeployment, error) {
	page, err := s.store.Pages().GetByID(ctx, pageID)
	if err != nil {
		return nil, err
	}
	if page.Status == model.PageStatusDeploying {
		return nil, fmt.Errorf("page is already deploying, wait for it to finish")
	}
	if page.CloudAccountID == nil {
		return nil, fmt.Errorf("configure a cloud account on this page before deploying")
	}
	if triggerType == "" {
		triggerType = "manual"
	}

	prevStatus := page.Status
	won, err := s.store.Pages().TryMarkDeploying(ctx, pageID)
	if err != nil {
		return nil, err
	}
	if !won {
		return nil, fmt.Errorf("page is already deploying, wait for it to finish")
	}
	page.Status = model.PageStatusDeploying

	dep := &model.PageDeployment{
		PageID:      pageID,
		Status:      model.PageDeployQueued,
		TriggerType: triggerType,
	}
	if err := s.store.PageDeployments().Create(ctx, dep); err != nil {
		s.setPageStatus(ctx, page, prevStatus)
		return nil, err
	}

	if err := s.queue.Enqueue(ctx, jobs.NewPageDeployJob(dep.ID)); err != nil {
		now := time.Now()
		dep.Status = model.PageDeployFailed
		dep.FinishedAt = &now
		dep.DeployLog = fmt.Sprintf("Failed to enqueue deploy job: %v", err)
		_ = s.store.PageDeployments().Update(ctx, dep)
		s.setPageStatus(ctx, page, prevStatus)
		return nil, fmt.Errorf("enqueue page deploy job: %w", err)
	}

	s.logger.Info("page deploy triggered", slog.String("page", page.Name), slog.String("deploy_id", dep.ID.String()))
	return dep, nil
}

// RunJob executes a queued Page deployment from the worker.
func (s *PageDeployService) RunJob(ctx context.Context, deploymentID uuid.UUID) error {
	dep, err := s.store.PageDeployments().GetByID(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load page deployment: %w", err)
	}
	if isTerminalPageStatus(dep.Status) {
		s.logger.Info("skipping page deploy — already terminal", slog.String("deploy_id", deploymentID.String()))
		return nil
	}
	page, err := s.store.Pages().GetByID(ctx, dep.PageID)
	if err != nil {
		return fmt.Errorf("load page: %w", err)
	}

	now := time.Now()
	dep.StartedAt = &now
	dep.Status = model.PageDeployDeploying
	// Bookkeeping (deploy row + page status + logs) is written under the parent
	// ctx, not a phase deadline, so a phase timeout still records the outcome
	// instead of leaving the deployment stuck in "deploying".
	s.updateDeploy(ctx, dep)
	s.setPageStatus(ctx, page, model.PageStatusDeploying)

	logf := func(line string) {
		dep.DeployLog = appendLog(dep.DeployLog, line)
		s.updateDeploy(ctx, dep)
	}

	// 1. Produce the directory of static files to sync. For build-enabled pages
	//    this runs an in-cluster build (clone + install + build) and uses the
	//    build output; otherwise it clones and resolves the pre-built publish
	//    folder. Both fail fast before any cloud work. This phase has its OWN
	//    time budget so a slow build doesn't consume the provisioning budget.
	prepCtx, cancelPrep := context.WithTimeout(ctx, pageBuildPhaseTimeout)
	filesDir, commitSHA, syncedPath, cleanup, err := s.prepareFiles(prepCtx, page, logf)
	cancelPrep()
	if err != nil {
		return s.fail(ctx, page, dep, err.Error())
	}
	defer cleanup()
	dep.CommitSHA = commitSHA
	dep.PublishPath = syncedPath
	s.updateDeploy(ctx, dep)

	// 2. Dedup — skip only when the last successful deploy used the SAME commit
	//    AND the SAME synced folder. Build-enabled pages skip dedup entirely:
	//    their output can change without a new commit (build env vars, command
	//    or dependency changes), so they always rebuild and re-sync.
	if !page.BuildEnabled && commitSHA != "" {
		if last, lerr := s.store.PageDeployments().GetLatestSuccess(ctx, page.ID); lerr == nil &&
			last != nil && last.CommitSHA == commitSHA && last.PublishPath == syncedPath {
			logf(fmt.Sprintf("Commit %s and publish folder %q unchanged since last successful deploy — skipping sync.", shortSHA(commitSHA), syncedPath))
			return s.succeed(ctx, page, dep, last.ProviderRef)
		}
	}

	// The cloud phase (provision + sync + invalidate) gets a fresh budget that
	// is independent of however long the build above took.
	jobCtx, cancel := context.WithTimeout(ctx, pageDeployPhaseTimeout)
	defer cancel()

	// 3. Resolve provider + credentials.
	provider, err := s.registry.Get(string(page.Provider))
	if err != nil {
		return s.fail(ctx, page, dep, err.Error())
	}
	cfg, err := resolveCloudConfig(jobCtx, s.store, page.CloudAccountID)
	if err != nil {
		return s.fail(ctx, page, dep, err.Error())
	}

	project, err := s.store.Projects().GetByID(jobCtx, page.ProjectID)
	if err != nil {
		return s.fail(ctx, page, dep, "load project: "+err.Error())
	}
	accountTags, err := resolveCloudAccountTags(jobCtx, s.store, page.CloudAccountID)
	if err != nil {
		return s.fail(ctx, page, dep, err.Error())
	}
	tagCtx := pages.TagContext{
		Env:     string(project.Environment),
		Project: project.Name,
		Page:    page.Name,
	}
	if project.Team != nil {
		tagCtx.Team = project.Team.Name
	}
	tagMap := pages.ResolveTags(accountTags, tagCtx)

	// Validate creds before provisioning (fail at connection check, not mid-create).
	if ok, msg, _ := provider.TestConnection(jobCtx, cfg); !ok {
		return s.fail(ctx, page, dep, "cloud credentials invalid: "+msg)
	}

	save := func(ctx context.Context, rt *model.PageRuntime) error {
		page.Runtime = rt
		return s.store.Pages().UpdateRuntime(ctx, page.ID, rt)
	}

	// Custom domain: AWS uses ACM + Route53 alias; Cloudflare uses Pages domain + CNAME.
	if strings.TrimSpace(page.CustomDomain) != "" && page.Provider == model.PageProviderAWSCloudFront {
		// Defense-in-depth against the "domain added after a no-domain deploy"
		// case: the distribution's Aliases/ViewerCertificate are only set at
		// creation, and Provision won't recreate an existing distribution. If a
		// distribution already exists but no cert was ever requested, the domain
		// was attached after the fact and can't be wired up — issuing a cert here
		// would leave the distribution serving 421s. The Update guard normally
		// blocks this; this catches any page already in that state.
		if page.Runtime != nil && page.Runtime.DistributionID != "" && page.Runtime.CertificateARN == "" {
			return s.fail(ctx, page, dep,
				"this page was first deployed without a custom domain; the existing CloudFront distribution cannot have a domain attached — recreate the page to use a custom domain")
		}
		if err := s.ensureCertificate(jobCtx, page, cfg, tagMap, save, logf); err != nil {
			return s.fail(ctx, page, dep, err.Error())
		}
	}

	// 4. Provision (idempotent; persists page.Runtime incrementally).
	if page.Provider == model.PageProviderAWSCloudFront && (page.Runtime == nil || page.Runtime.DistributionID == "") {
		logf("Provisioning S3 + CloudFront and waiting for the distribution to deploy (first deploy can take ~10–15 min)…")
	} else if page.Provider == model.PageProviderCloudflarePages && (page.Runtime == nil || page.Runtime.CFProjectID == "") {
		logf("Provisioning Cloudflare Pages project…")
	}
	rt, err := provider.Provision(jobCtx, page, cfg, tagMap, save)
	if err != nil {
		return s.fail(ctx, page, dep, "provisioning failed: "+err.Error())
	}
	page.Runtime = rt

	// Final DNS for custom domain.
	if strings.TrimSpace(page.CustomDomain) != "" && page.Runtime.DefaultURL != "" {
		switch page.Provider {
		case model.PageProviderAWSCloudFront:
			if err := s.ensureAliasRecord(jobCtx, page, logf); err != nil {
				return s.fail(ctx, page, dep, "alias DNS record failed: "+err.Error())
			}
		case model.PageProviderCloudflarePages:
			if err := s.ensureCloudflarePagesDomain(jobCtx, page, cfg, logf); err != nil {
				return s.fail(ctx, page, dep, "custom domain setup failed: "+err.Error())
			}
		}
	}

	// 5. Deploy (sync + invalidate / direct upload).
	result, err := provider.Deploy(jobCtx, page, cfg, filesDir, logf)
	if err != nil {
		return s.fail(ctx, page, dep, "deploy failed: "+err.Error())
	}
	if result.DefaultURL != "" {
		if page.CustomDomain != "" {
			logf("Live at https://" + page.CustomDomain)
		} else {
			logf("Live at " + result.DefaultURL)
		}
	}
	return s.succeed(ctx, page, dep, result.ProviderRef)
}

// prepareFiles produces the local directory of static files to sync, returning
// the directory, the built/cloned commit SHA, the folder path snapshot (for the
// deployment record + dedup), and a cleanup func. It branches on whether the
// page builds from source or syncs pre-built files.
func (s *PageDeployService) prepareFiles(ctx context.Context, page *model.Page, logf func(string)) (filesDir, commitSHA, syncedPath string, cleanup func(), err error) {
	if page.BuildEnabled {
		return s.buildFiles(ctx, page, logf)
	}

	src, perr := s.publishSvc.Prepare(ctx, page, logf)
	if perr != nil {
		return "", "", "", func() {}, fmt.Errorf("clone failed: %w", perr)
	}
	return src.FilesDir, src.CommitSHA, src.PublishPath, src.Cleanup, nil
}

// buildFiles runs an in-cluster build of the page and returns the built output
// directory. The synced-path snapshot is "root_directory/output_dir" so a
// non-build → build switch (or an output-dir change) is reflected in the
// deployment record.
func (s *PageDeployService) buildFiles(ctx context.Context, page *model.Page, logf func(string)) (filesDir, commitSHA, syncedPath string, cleanup func(), err error) {
	builder, berr := orchestrator.AsCapability[orchestrator.StaticSiteBuilder](s.targets.Default(), orchestrator.CapPageBuild)
	if berr != nil {
		return "", "", "", func() {}, fmt.Errorf("static-site builds are not supported by the deploy target: %w", berr)
	}

	// Token is required only for private repos; let public repos clone without
	// one rather than failing when no provider is linked.
	token, terr := s.publishSvc.ResolveGitToken(ctx, page)
	if terr != nil && page.GitProviderID != nil {
		return "", "", "", func() {}, fmt.Errorf("resolve git token: %w", terr)
	}

	logf("Building site in an in-cluster build pod…")
	result, rerr := builder.BuildStatic(ctx, orchestrator.StaticBuildOpts{
		GitRepo:        page.GitRepo,
		GitBranch:      page.GitBranch,
		GitToken:       token,
		PageID:         page.ID.String(),
		RootDirectory:  page.RootDirectory,
		PackageManager: page.PackageManager,
		InstallCommand: page.InstallCommand,
		BuildCommand:   page.BuildCommand,
		OutputDir:      page.OutputDir,
		BuildEnvVars:   page.BuildEnvVars,
		NodeImage:      nodeImageForVersion(page.NodeVersion),
		OnLog:          logf,
	})
	if rerr != nil {
		return "", "", "", func() {}, fmt.Errorf("build failed: %w", rerr)
	}

	cleanupFn := result.Cleanup
	if cleanupFn == nil {
		cleanupFn = func() {}
	}
	return result.FilesDir, result.CommitSHA, buildSyncedPath(page), cleanupFn, nil
}

// buildSyncedPath is the snapshot path recorded on a build deployment.
func buildSyncedPath(page *model.Page) string {
	root := strings.TrimSpace(page.RootDirectory)
	if root == "" {
		root = "."
	}
	out := strings.TrimSpace(page.OutputDir)
	if out == "" {
		return root
	}
	if root == "." {
		return out
	}
	return root + "/" + out
}

// nodeImageForVersion maps a node major version (e.g. "20") to a build image.
// Empty means use the builder's default image.
func nodeImageForVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	return "node:" + version + "-bookworm-slim"
}

// ensureCertificate requests an ACM cert, creates validation DNS when configured,
// and waits for ISSUED before CloudFront can use the alias.
func (s *PageDeployService) ensureCertificate(
	ctx context.Context,
	page *model.Page,
	cfg json.RawMessage,
	tags map[string]string,
	save pages.SaveRuntime,
	logf func(string),
) error {
	cloudCreds, err := pages.ParseCredentials(cfg)
	if err != nil {
		return err
	}
	if page.Runtime == nil {
		page.Runtime = &model.PageRuntime{}
	}
	rt := page.Runtime

	// Once the certificate is ISSUED nothing here needs to run again: the cert is
	// permanent, the validation CNAME has served its purpose, and CloudFront
	// already references the ARN. Re-running ACM/Route53 calls on every redeploy
	// would otherwise fail all future deploys the moment DNS-account credentials
	// rotate or expire.
	if rt.CertificateARN != "" && rt.CertStatus == "ISSUED" {
		return nil
	}

	acmClient := acm.New()
	domain := normalizeDomain(page.CustomDomain)

	if rt.CertificateARN == "" {
		logf("Requesting ACM certificate for " + domain + " (us-east-1)…")
		token := strings.ReplaceAll(page.ID.String(), "-", "")
		arn, err := acmClient.RequestCertificate(ctx, cloudCreds, domain, token, tags)
		if err != nil {
			return err
		}
		rt.CertificateARN = arn
		rt.CertStatus = "PENDING_VALIDATION"
		if err := save(ctx, rt); err != nil {
			return fmt.Errorf("persist certificate ARN: %w", err)
		}
	}

	name, typ, value, err := acmClient.ValidationRecord(ctx, cloudCreds, rt.CertificateARN)
	if err != nil {
		return err
	}
	rt.ValidationRecord = &model.PageValidationRecord{Name: name, Type: typ, Value: value}
	if err := save(ctx, rt); err != nil {
		return fmt.Errorf("persist validation record: %w", err)
	}
	logf(fmt.Sprintf("ACM validation record: %s %s → %s", typ, name, value))

	if page.ManageDNS {
		if page.DNSAccountID == nil || strings.TrimSpace(page.DNSZoneID) == "" {
			return fmt.Errorf("dns_account_id and dns_zone_id are required when manage_dns is enabled")
		}
		// Fail fast before any ACM work: CloudFront-hosted pages need a Route53
		// DNS account (Cloudflare cannot create the alias record later).
		if err := validatePagesDNSAccount(ctx, s.store, page.Provider, *page.DNSAccountID); err != nil {
			return err
		}
		prov, dnsCfg, err := resolveDNSProvider(ctx, s.store, *page.DNSAccountID)
		if err != nil {
			return err
		}
		logf("Creating ACM validation record in Route53…")
		if err := prov.UpsertRecord(ctx, dnsCfg, page.DNSZoneID, dns.Record{
			Name:   name,
			Type:   typ,
			TTL:    300,
			Values: []string{value},
		}); err != nil {
			return fmt.Errorf("create validation DNS record: %w", err)
		}
		logf("Waiting for ACM certificate to become ISSUED (up to 10 min)…")
		if err := acmClient.WaitIssued(ctx, cloudCreds, rt.CertificateARN, acmCertWaitTimeout); err != nil {
			return err
		}
		rt.CertStatus = "ISSUED"
		return save(ctx, rt)
	}

	issued, err := acmClient.IsIssued(ctx, cloudCreds, rt.CertificateARN)
	if err != nil {
		return err
	}
	if issued {
		rt.CertStatus = "ISSUED"
		return save(ctx, rt)
	}

	logf("ACTION REQUIRED: Add the ACM validation CNAME record shown above to your DNS provider, then re-deploy.")
	return fmt.Errorf("ACM certificate is not issued yet — add the validation DNS record and re-deploy")
}

// ensureAliasRecord creates the Route53 alias pointing the custom domain at CloudFront.
func (s *PageDeployService) ensureAliasRecord(ctx context.Context, page *model.Page, logf func(string)) error {
	target := cloudFrontDomainFromURL(page.Runtime.DefaultURL)
	// Once alias records exist for this CloudFront target, skip Route53 on redeploy.
	// The target is stable for the life of the distribution; re-upserting would fail
	// if DNS-account credentials rotate or expire after the first successful deploy.
	if page.ManageDNS && page.Runtime.AliasTarget == target {
		return nil
	}

	page.Runtime.AliasTarget = target
	if err := s.store.Pages().UpdateRuntime(ctx, page.ID, page.Runtime); err != nil {
		return err
	}

	if !page.ManageDNS {
		logf(fmt.Sprintf("Create a Route53 ALIAS (or CNAME) record: %s → %s (zone %s)", page.CustomDomain, target, dns.CloudFrontHostedZoneID))
		return nil
	}
	if page.DNSAccountID == nil || strings.TrimSpace(page.DNSZoneID) == "" {
		return fmt.Errorf("dns_account_id and dns_zone_id are required when manage_dns is enabled")
	}
	// Defensive guard: CloudFront alias records are Route53-only. Reject non-AWS
	// DNS accounts before attempting upserts so a Cloudflare account fails with a
	// clear message instead of an opaque "alias not supported" error mid-deploy.
	if err := validatePagesDNSAccount(ctx, s.store, page.Provider, *page.DNSAccountID); err != nil {
		return err
	}
	prov, dnsCfg, err := resolveDNSProvider(ctx, s.store, *page.DNSAccountID)
	if err != nil {
		return err
	}
	alias := &dns.Alias{
		TargetZoneID:         dns.CloudFrontHostedZoneID,
		TargetDNSName:        target,
		EvaluateTargetHealth: false,
	}
	for _, typ := range []string{"A", "AAAA"} {
		if err := prov.UpsertRecord(ctx, dnsCfg, page.DNSZoneID, dns.Record{
			Name:  page.CustomDomain,
			Type:  typ,
			Alias: alias,
		}); err != nil {
			return fmt.Errorf("create %s alias: %w", typ, err)
		}
	}
	logf("Route53 ALIAS records created for " + page.CustomDomain)
	return nil
}

func (s *PageDeployService) ensureCloudflarePagesDomain(
	ctx context.Context,
	page *model.Page,
	cfg json.RawMessage,
	logf func(string),
) error {
	if page.Runtime == nil || page.Runtime.CFProjectName == "" {
		return fmt.Errorf("Cloudflare Pages project is not provisioned")
	}
	target := pagesDevHostFromURL(page.Runtime.DefaultURL)
	if target == "" {
		return fmt.Errorf("missing default Pages URL")
	}

	if page.Runtime.AliasTarget == target {
		return nil
	}

	provider, err := s.registry.Get(string(page.Provider))
	if err != nil {
		return err
	}
	attacher, ok := provider.(pages.CustomDomainAttacher)
	if !ok {
		return fmt.Errorf("pages provider %q does not support custom domain attachment", page.Provider)
	}
	logf("Attaching custom domain " + page.CustomDomain + " to Cloudflare Pages…")
	if err := attacher.AttachCustomDomain(ctx, cfg, page.Runtime.CFProjectName, page.CustomDomain); err != nil {
		return fmt.Errorf("attach Pages custom domain: %w", err)
	}

	page.Runtime.AliasTarget = target
	if err := s.store.Pages().UpdateRuntime(ctx, page.ID, page.Runtime); err != nil {
		return err
	}

	if !page.ManageDNS {
		logf(fmt.Sprintf("Create a CNAME record: %s → %s", page.CustomDomain, target))
		return nil
	}
	if page.DNSAccountID == nil || strings.TrimSpace(page.DNSZoneID) == "" {
		return fmt.Errorf("dns_account_id and dns_zone_id are required when manage_dns is enabled")
	}
	if err := validatePagesDNSAccount(ctx, s.store, page.Provider, *page.DNSAccountID); err != nil {
		return err
	}
	prov, dnsCfg, err := resolveDNSProvider(ctx, s.store, *page.DNSAccountID)
	if err != nil {
		return err
	}
	logf("Creating CNAME record in Cloudflare DNS…")
	if err := prov.UpsertRecord(ctx, dnsCfg, page.DNSZoneID, dns.Record{
		Name:   page.CustomDomain,
		Type:   "CNAME",
		TTL:    300,
		Values: []string{target},
	}); err != nil {
		return fmt.Errorf("create CNAME record: %w", err)
	}
	logf("Cloudflare DNS CNAME created for " + page.CustomDomain)
	return nil
}

func (s *PageDeployService) succeed(ctx context.Context, page *model.Page, dep *model.PageDeployment, providerRef string) error {
	now := time.Now()
	dep.Status = model.PageDeploySuccess
	dep.ProviderRef = providerRef
	dep.FinishedAt = &now
	s.updateDeploy(ctx, dep)
	s.setPageStatus(ctx, page, model.PageStatusLive)
	s.logger.Info("page deploy succeeded", slog.String("page", page.Name), slog.String("deploy_id", dep.ID.String()))
	s.notify(ctx, page, model.EventDeploySuccess, fmt.Sprintf("Page %s deployed", page.Name))
	return nil
}

func (s *PageDeployService) fail(ctx context.Context, page *model.Page, dep *model.PageDeployment, msg string) error {
	now := time.Now()
	dep.Status = model.PageDeployFailed
	dep.FinishedAt = &now
	dep.DeployLog = appendLog(dep.DeployLog, "ERROR: "+msg)
	s.updateDeploy(ctx, dep)
	s.setPageStatus(ctx, page, model.PageStatusError)
	s.logger.Error("page deploy failed", slog.String("page", page.Name), slog.String("error", msg))
	s.notify(ctx, page, model.EventDeployFailed, fmt.Sprintf("Page %s deploy failed: %s", page.Name, msg))
	// Returning nil: the failure is recorded on the deployment; we don't want the
	// worker to retry a user-fixable error (bad creds, missing folder) 5×.
	return nil
}

func (s *PageDeployService) setPageStatus(ctx context.Context, page *model.Page, status model.PageStatus) {
	if page.Status == status {
		return
	}
	page.Status = status
	if err := s.store.Pages().UpdateStatus(ctx, page.ID, status); err != nil {
		s.logger.Error("failed to update page status", slog.String("page", page.Name), slog.Any("error", err))
	}
}

func (s *PageDeployService) updateDeploy(ctx context.Context, dep *model.PageDeployment) {
	if err := s.store.PageDeployments().Update(ctx, dep); err != nil {
		s.logger.Error("failed to update page deployment", slog.String("deploy_id", dep.ID.String()), slog.Any("error", err))
	}
}

func (s *PageDeployService) notify(ctx context.Context, page *model.Page, event model.NotifyEvent, detail string) {
	if s.notifSvc == nil {
		return
	}
	project, err := s.store.Projects().GetByID(ctx, page.ProjectID)
	if err != nil {
		return
	}
	s.notifSvc.NotifyAsync(project.OrgID, event, fmt.Sprintf("%s: %s", page.Name, string(event)), detail)
}

func isTerminalPageStatus(status model.PageDeploymentStatus) bool {
	switch status {
	case model.PageDeploySuccess, model.PageDeployFailed, model.PageDeployCancelled:
		return true
	default:
		return false
	}
}

func appendLog(existing, line string) string {
	stamp := time.Now().UTC().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s", stamp, line)
	if existing == "" {
		return entry
	}
	return strings.Join([]string{existing, entry}, "\n")
}
