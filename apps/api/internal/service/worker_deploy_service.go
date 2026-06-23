package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/workers"
)

// workerDeployPhaseTimeout bounds a single worker build+deploy (clone + install
// + wrangler deploy). The in-pod build caps itself (see k3s workerBuildTimeout);
// this adds headroom for pod scheduling and image pull.
const workerDeployPhaseTimeout = 30 * time.Minute

// WorkerDeployService triggers and runs Cloudflare Worker deployments
// (clone → install → wrangler deploy in an in-cluster build pod).
type WorkerDeployService struct {
	store     store.Store
	notifSvc  *NotificationService
	queue     Enqueuer
	targets   *orchestrator.TargetRegistry
	providers *providers.Registry
	logger    *slog.Logger
}

func NewWorkerDeployService(
	s store.Store,
	notifSvc *NotificationService,
	queue Enqueuer,
	targets *orchestrator.TargetRegistry,
	prov *providers.Registry,
	logger *slog.Logger,
) *WorkerDeployService {
	return &WorkerDeployService{
		store:     s,
		notifSvc:  notifSvc,
		queue:     queue,
		targets:   targets,
		providers: prov,
		logger:    logger,
	}
}

// StartStaleRecovery marks Worker deployments stuck "deploying" for >40 min as
// failed, on startup and every 5 minutes. When isLeader is non-nil, only the
// elected leader runs recovery.
func (s *WorkerDeployService) StartStaleRecovery(ctx context.Context, isLeader func() bool) {
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

func (s *WorkerDeployService) recoverStale() {
	ctx := context.Background()
	threshold := 40 * time.Minute
	deps, _, err := s.store.WorkerDeployments().ListByStatus(ctx, model.WorkerDeployDeploying, store.ListParams{Page: 1, PerPage: 1000})
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
		marked, err := s.store.WorkerDeployments().MarkTimedOut(ctx, d.ID, time.Now(), "\n\n--- Deploy timed out (stale for >40min) ---")
		if err != nil || !marked {
			continue
		}
		if uerr := s.store.Workers().UpdateStatus(ctx, d.WorkerID, model.WorkerStatusError); uerr != nil {
			s.logger.Error("stale recovery: failed to mark worker errored",
				slog.String("worker_id", d.WorkerID.String()), slog.Any("error", uerr))
		}
		s.logger.Warn("recovered stale worker deployment", slog.String("deploy_id", d.ID.String()))
	}
}

// Trigger creates a queued deployment and enqueues the job.
func (s *WorkerDeployService) Trigger(ctx context.Context, workerID uuid.UUID, triggerType string) (*model.WorkerDeployment, error) {
	worker, err := s.store.Workers().GetByID(ctx, workerID)
	if err != nil {
		return nil, err
	}
	if worker.Status == model.WorkerStatusDeploying {
		return nil, apierr.ErrConflict.WithDetail("worker is already deploying, wait for it to finish")
	}
	if worker.CloudAccountID == nil {
		return nil, fmt.Errorf("configure a Cloudflare cloud account on this worker before deploying")
	}
	if triggerType == "" {
		triggerType = "manual"
	}

	prevStatus := worker.Status
	won, err := s.store.Workers().TryMarkDeploying(ctx, workerID)
	if err != nil {
		return nil, err
	}
	if !won {
		return nil, apierr.ErrConflict.WithDetail("worker is already deploying, wait for it to finish")
	}
	worker.Status = model.WorkerStatusDeploying

	dep := &model.WorkerDeployment{
		WorkerID:    workerID,
		Status:      model.WorkerDeployQueued,
		TriggerType: triggerType,
	}
	if err := s.store.WorkerDeployments().Create(ctx, dep); err != nil {
		s.setWorkerStatus(ctx, worker, prevStatus)
		return nil, err
	}

	if err := s.queue.Enqueue(ctx, jobs.NewWorkerDeployJob(dep.ID)); err != nil {
		now := time.Now()
		dep.Status = model.WorkerDeployFailed
		dep.FinishedAt = &now
		dep.DeployLog = fmt.Sprintf("Failed to enqueue deploy job: %v", err)
		_ = s.store.WorkerDeployments().Update(ctx, dep)
		s.setWorkerStatus(ctx, worker, prevStatus)
		return nil, fmt.Errorf("enqueue worker deploy job: %w", err)
	}

	s.logger.Info("worker deploy triggered", slog.String("worker", worker.Name), slog.String("deploy_id", dep.ID.String()))
	return dep, nil
}

// RunJob executes a queued Worker deployment from the worker process.
func (s *WorkerDeployService) RunJob(ctx context.Context, deploymentID uuid.UUID) error {
	dep, err := s.store.WorkerDeployments().GetByID(ctx, deploymentID)
	if err != nil {
		return fmt.Errorf("load worker deployment: %w", err)
	}
	if isTerminalWorkerStatus(dep.Status) {
		s.logger.Info("skipping worker deploy — already terminal", slog.String("deploy_id", deploymentID.String()))
		return nil
	}
	worker, err := s.store.Workers().GetByID(ctx, dep.WorkerID)
	if err != nil {
		return fmt.Errorf("load worker: %w", err)
	}

	now := time.Now()
	dep.StartedAt = &now
	dep.Status = model.WorkerDeployDeploying
	s.updateDeploy(ctx, dep)
	s.setWorkerStatus(ctx, worker, model.WorkerStatusDeploying)

	logf := func(line string) {
		dep.DeployLog = appendLog(dep.DeployLog, line)
		s.updateDeploy(ctx, dep)
	}

	// Resolve Cloudflare deploy credentials from the linked cloud account.
	cfg, err := resolveCloudConfig(ctx, s.store, worker.CloudAccountID)
	if err != nil {
		return s.fail(ctx, worker, dep, err.Error())
	}
	apiToken, accountID, err := workers.DeployCredentials(cfg)
	if err != nil {
		return s.fail(ctx, worker, dep, err.Error())
	}

	// Git token is required only for private repos; let public repos clone
	// without one rather than failing when no provider is linked.
	token, terr := resolveWorkerGitToken(ctx, s.providers, s.store, worker)
	if terr != nil && worker.GitProviderID != nil {
		return s.fail(ctx, worker, dep, "resolve git token: "+terr.Error())
	}

	builder, berr := orchestrator.AsCapability[orchestrator.WorkerBuilder](s.targets.Default(), orchestrator.CapWorkerBuild)
	if berr != nil {
		return s.fail(ctx, worker, dep, "worker deploys are not supported by the deploy target: "+berr.Error())
	}

	jobCtx, cancel := context.WithTimeout(ctx, workerDeployPhaseTimeout)
	defer cancel()

	logf("Deploying worker in an in-cluster build pod…")
	result, rerr := builder.BuildWorker(jobCtx, orchestrator.WorkerBuildOpts{
		GitRepo:            worker.GitRepo,
		GitBranch:          worker.GitBranch,
		GitToken:           token,
		WorkerID:           worker.ID.String(),
		RootDirectory:      worker.RootDirectory,
		WranglerConfig:     worker.WranglerConfig,
		PackageManager:     worker.PackageManager,
		InstallCommand:     worker.InstallCommand,
		BuildCommand:       worker.BuildCommand,
		DeployCommand:      worker.DeployCommand,
		BuildEnvVars:       worker.BuildEnvVars,
		R2ConfirmedBuckets: worker.R2ConfirmedBuckets,
		CFAPIToken:         apiToken,
		CFAccountID:        accountID,
		OnLog:              logf,
	})
	if rerr != nil {
		return s.fail(ctx, worker, dep, "deploy failed: "+rerr.Error())
	}

	// The build pod stopped before deploying because an R2 bucket referenced by
	// the wrangler config already exists and the user hasn't approved its reuse.
	// Park the deploy in needs_confirmation; ConfirmR2 resumes it.
	if result.NeedsR2Confirmation {
		if result.CommitSHA != "" {
			dep.CommitSHA = result.CommitSHA
		}
		return s.needsConfirmation(ctx, worker, dep, result.PendingR2Buckets)
	}

	// Persist the discovered runtime (script name + live URL).
	rt := worker.Runtime
	if rt == nil {
		rt = &model.WorkerRuntime{}
	}
	if result.ScriptName != "" {
		rt.ScriptName = result.ScriptName
	}
	if result.DeployedURL != "" {
		rt.DeployedURL = result.DeployedURL
	}
	if result.DeployID != "" {
		rt.LastDeployID = result.DeployID
	}
	worker.Runtime = rt
	if err := s.store.Workers().UpdateRuntime(ctx, worker.ID, rt); err != nil {
		s.logger.Error("failed to persist worker runtime", slog.String("worker", worker.Name), slog.Any("error", err))
	}

	if result.CommitSHA != "" {
		dep.CommitSHA = result.CommitSHA
	}
	if rt.DeployedURL != "" {
		logf("Live at " + rt.DeployedURL)
	}
	return s.succeed(ctx, worker, dep, rt.ScriptName)
}

func (s *WorkerDeployService) succeed(ctx context.Context, worker *model.Worker, dep *model.WorkerDeployment, providerRef string) error {
	now := time.Now()
	dep.Status = model.WorkerDeploySuccess
	dep.ProviderRef = providerRef
	dep.FinishedAt = &now
	s.updateDeploy(ctx, dep)
	s.setWorkerStatus(ctx, worker, model.WorkerStatusLive)
	s.logger.Info("worker deploy succeeded", slog.String("worker", worker.Name), slog.String("deploy_id", dep.ID.String()))
	s.notify(ctx, worker, model.EventDeploySuccess, fmt.Sprintf("Worker %s deployed", worker.Name))
	return nil
}

// needsConfirmation parks a deploy that detected pre-existing R2 buckets. The
// worker returns to idle so the user can confirm (ConfirmR2) and re-deploy.
func (s *WorkerDeployService) needsConfirmation(ctx context.Context, worker *model.Worker, dep *model.WorkerDeployment, pending []model.WorkerR2Bucket) error {
	now := time.Now()
	dep.Status = model.WorkerDeployNeedsConfirmation
	dep.R2Pending = pending
	dep.FinishedAt = &now
	names := make([]string, len(pending))
	for i, b := range pending {
		names[i] = b.Name
	}
	dep.DeployLog = appendLog(dep.DeployLog,
		fmt.Sprintf("Paused: R2 bucket(s) already exist and need confirmation: %s", strings.Join(names, ", ")))
	s.updateDeploy(ctx, dep)
	s.setWorkerStatus(ctx, worker, model.WorkerStatusIdle)
	s.logger.Info("worker deploy needs R2 confirmation",
		slog.String("worker", worker.Name), slog.String("deploy_id", dep.ID.String()), slog.Int("pending", len(pending)))
	return nil
}

// ConfirmR2 approves the pre-existing R2 buckets from the worker's latest
// needs_confirmation deployment and re-triggers the deploy.
func (s *WorkerDeployService) ConfirmR2(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error) {
	worker, err := s.store.Workers().GetByID(ctx, workerID)
	if err != nil {
		return nil, err
	}
	dep, err := s.store.WorkerDeployments().GetLatestByWorker(ctx, workerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apierr.ErrNotFound.WithDetail("no deployment awaiting confirmation")
		}
		return nil, fmt.Errorf("get latest worker deployment: %w", err)
	}
	if dep.Status != model.WorkerDeployNeedsConfirmation {
		return nil, apierr.ErrConflict.WithDetail("no deployment is awaiting R2 confirmation")
	}

	// Atomically claim the parked deployment before persisting buckets or
	// triggering a replacement. Without this, concurrent ConfirmR2 calls can
	// both pass the status check and enqueue duplicate deploys.
	claimed, err := s.store.WorkerDeployments().TryClaimNeedsConfirmation(ctx, dep.ID)
	if err != nil {
		return nil, fmt.Errorf("claim R2 confirmation deployment: %w", err)
	}
	if !claimed {
		return nil, apierr.ErrConflict.WithDetail("no deployment is awaiting R2 confirmation")
	}
	origFinishedAt := dep.FinishedAt
	dep.Status = model.WorkerDeployCancelled

	confirmed := append([]string{}, worker.R2ConfirmedBuckets...)
	seen := make(map[string]bool, len(confirmed))
	for _, n := range confirmed {
		seen[n] = true
	}
	for _, b := range dep.R2Pending {
		if !seen[b.Name] {
			confirmed = append(confirmed, b.Name)
			seen[b.Name] = true
		}
	}
	worker.R2ConfirmedBuckets = confirmed
	if err := s.store.Workers().Update(ctx, worker); err != nil {
		s.revertR2ConfirmationClaim(ctx, dep, origFinishedAt, "Failed to persist confirmed R2 buckets.")
		return nil, fmt.Errorf("persist confirmed R2 buckets: %w", err)
	}

	// Trigger the replacement deployment, then finalize the parked one. If
	// Trigger fails, restore needs_confirmation so the banner persists.
	newDep, err := s.Trigger(ctx, workerID, "manual")
	if err != nil {
		s.revertR2ConfirmationClaim(ctx, dep, origFinishedAt, "R2 confirmation failed; please try again.")
		return nil, err
	}

	now := time.Now()
	dep.FinishedAt = &now
	dep.DeployLog = appendLog(dep.DeployLog, "R2 bucket(s) confirmed; re-deploying.")
	s.updateDeploy(ctx, dep)

	return newDep, nil
}

func (s *WorkerDeployService) revertR2ConfirmationClaim(ctx context.Context, dep *model.WorkerDeployment, finishedAt *time.Time, msg string) {
	dep.Status = model.WorkerDeployNeedsConfirmation
	dep.FinishedAt = finishedAt
	dep.DeployLog = appendLog(dep.DeployLog, msg)
	s.updateDeploy(ctx, dep)
}

func (s *WorkerDeployService) fail(ctx context.Context, worker *model.Worker, dep *model.WorkerDeployment, msg string) error {
	now := time.Now()
	dep.Status = model.WorkerDeployFailed
	dep.FinishedAt = &now
	dep.DeployLog = appendLog(dep.DeployLog, "ERROR: "+msg)
	s.updateDeploy(ctx, dep)
	s.setWorkerStatus(ctx, worker, model.WorkerStatusError)
	s.logger.Error("worker deploy failed", slog.String("worker", worker.Name), slog.String("error", msg))
	s.notify(ctx, worker, model.EventDeployFailed, fmt.Sprintf("Worker %s deploy failed: %s", worker.Name, msg))
	// Returning nil: the failure is recorded; we don't want the worker process
	// to retry a user-fixable error (bad creds, missing config) 5×.
	return nil
}

func (s *WorkerDeployService) setWorkerStatus(ctx context.Context, worker *model.Worker, status model.WorkerStatus) {
	if worker.Status == status {
		return
	}
	worker.Status = status
	if err := s.store.Workers().UpdateStatus(ctx, worker.ID, status); err != nil {
		s.logger.Error("failed to update worker status", slog.String("worker", worker.Name), slog.Any("error", err))
	}
}

func (s *WorkerDeployService) updateDeploy(ctx context.Context, dep *model.WorkerDeployment) {
	if err := s.store.WorkerDeployments().Update(ctx, dep); err != nil {
		s.logger.Error("failed to update worker deployment", slog.String("deploy_id", dep.ID.String()), slog.Any("error", err))
	}
}

func (s *WorkerDeployService) notify(ctx context.Context, worker *model.Worker, event model.NotifyEvent, detail string) {
	if s.notifSvc == nil {
		return
	}
	project, err := s.store.Projects().GetByID(ctx, worker.ProjectID)
	if err != nil {
		return
	}
	s.notifSvc.NotifyAsync(project.OrgID, event, fmt.Sprintf("%s: %s", worker.Name, string(event)), detail)
}

func isTerminalWorkerStatus(status model.WorkerDeploymentStatus) bool {
	switch status {
	case model.WorkerDeploySuccess, model.WorkerDeployFailed, model.WorkerDeployCancelled,
		model.WorkerDeployNeedsConfirmation:
		return true
	default:
		return false
	}
}
