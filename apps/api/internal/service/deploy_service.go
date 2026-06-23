package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

const deployJobTimeout = 30 * time.Minute

type DeployService struct {
	store        store.Store
	targets      *orchestrator.TargetRegistry
	logger       *slog.Logger
	buildSvc     *BuildService
	notifSvc     *NotificationService
	registryAuth *RegistryAuth
	queue        Enqueuer
}

func NewDeployService(
	s store.Store,
	targets *orchestrator.TargetRegistry,
	logger *slog.Logger,
	buildSvc *BuildService,
	notifSvc *NotificationService,
	registryAuth *RegistryAuth,
	queue Enqueuer,
) *DeployService {
	return &DeployService{
		store:        s,
		targets:      targets,
		logger:       logger,
		buildSvc:     buildSvc,
		notifSvc:     notifSvc,
		registryAuth: registryAuth,
		queue:        queue,
	}
}

// StartStaleRecovery runs stale deployment recovery on startup and every 5 minutes.
// When isLeader is non-nil, only the elected leader recovers stale deployments.
func (s *DeployService) StartStaleRecovery(ctx context.Context, isLeader func() bool) {
	go func() {
		if shouldRunSingleton(isLeader) {
			s.recoverStaleDeployments(ctx)
		}
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if shouldRunSingleton(isLeader) {
					s.recoverStaleDeployments(ctx)
				}
			}
		}
	}()
}

// recoverStaleDeployments marks deployments stuck for >30 minutes as failed.
// "queued" is intentionally excluded: with the PGMQ backlog a deployment can sit
// queued behind other jobs well past 30 minutes and still be perfectly valid —
// only actively-running ("building"/"deploying") deploys can be truly orphaned.
func (s *DeployService) recoverStaleDeployments(ctx context.Context) {
	staleThreshold := 30 * time.Minute
	const perPage = 1000
	for _, status := range []string{"building", "deploying"} {
		for page := 1; ; page++ {
			deploys, _, err := s.store.Deployments().ListAll(ctx, store.ListParams{Page: page, PerPage: perPage}, store.DeploymentListFilter{Status: status})
			if err != nil {
				break
			}
			if len(deploys) == 0 {
				break
			}
			for _, d := range deploys {
				ref := d.CreatedAt
				if d.StartedAt != nil {
					ref = *d.StartedAt
				}
				age := time.Since(ref)
				if age < staleThreshold {
					s.logger.Info("skipping recent in-progress deployment",
						slog.String("deploy_id", d.ID.String()),
						slog.String("app_name", d.AppName),
						slog.Duration("age", age),
					)
					continue
				}
				now := time.Now()
				d.Status = model.DeployFailed
				d.FinishedAt = &now
				d.BuildLog += "\n\n--- Deployment timed out (stale for >30min) ---"
				s.updateDeploy(ctx, &d)
				s.setAppStatus(ctx, d.AppID, model.AppStatusError)
				s.logger.Warn("recovered stale deployment",
					slog.String("deploy_id", d.ID.String()),
					slog.String("app_name", d.AppName),
					slog.String("was_status", status),
					slog.Duration("age", age),
				)
			}
			if len(deploys) < perPage {
				break
			}
		}
	}
}

type TriggerDeployInput struct {
	AppID       uuid.UUID  `json:"app_id" binding:"required"`
	ForceBuild  bool       `json:"force_build"`
	TriggeredBy *uuid.UUID `json:"-"`
	TriggerType string     `json:"-"` // manual | webhook | rollback
}

// Trigger creates a new deployment record and enqueues the build/deploy job.
func (s *DeployService) Trigger(ctx context.Context, input TriggerDeployInput) (*model.Deployment, error) {
	app, err := s.store.Applications().GetByID(ctx, input.AppID)
	if err != nil {
		return nil, err
	}

	switch app.Status {
	case model.AppStatusBuilding, model.AppStatusDeploying, model.AppStatusRestarting, model.AppStatusStopping:
		return nil, fmt.Errorf("app is currently %s, wait for it to finish first", app.Status)
	}

	deploy := &model.Deployment{
		AppID:       app.ID,
		Status:      model.DeployQueued,
		TriggerType: input.TriggerType,
		TriggeredBy: input.TriggeredBy,
		AppName:     app.Name,
		ProjectID:   app.ProjectID,
	}

	if err := s.store.Deployments().Create(ctx, deploy); err != nil {
		return nil, err
	}

	// Only mark the app building once the job is durably enqueued. If the enqueue
	// fails the app stays in its prior state instead of being stuck "building"
	// with no work actually running.
	if err := s.queue.Enqueue(ctx, jobs.NewDeployJob(deploy.ID, input.ForceBuild)); err != nil {
		// The deploy row was already created. recoverStaleDeployments intentionally
		// ignores "queued", so without finalizing it here a failed enqueue would
		// leave a permanent queued orphan. Mark it failed immediately.
		now := time.Now()
		deploy.Status = model.DeployFailed
		deploy.FinishedAt = &now
		deploy.BuildLog = fmt.Sprintf("Failed to enqueue deploy job: %v", err)
		s.updateDeploy(ctx, deploy)
		return nil, fmt.Errorf("enqueue deploy job: %w", err)
	}

	s.setAppStatus(ctx, app.ID, model.AppStatusBuilding)

	s.logger.Info("deployment triggered",
		slog.String("deployment_id", deploy.ID.String()),
		slog.String("app", app.Name),
		slog.String("trigger", input.TriggerType),
	)

	return deploy, nil
}

// RunDeployJob executes a deploy job from the worker queue.
func (s *DeployService) RunDeployJob(ctx context.Context, deployID uuid.UUID, forceBuild bool) error {
	deploy, err := s.store.Deployments().GetByID(ctx, deployID)
	if err != nil {
		return fmt.Errorf("load deployment: %w", err)
	}

	if isTerminalDeployStatus(deploy.Status) {
		s.logger.Info("skipping deploy job — already terminal",
			slog.String("deploy_id", deployID.String()),
			slog.String("status", string(deploy.Status)),
		)
		return nil
	}

	if deploy.CancelRequested {
		app, _ := s.store.Applications().GetByID(ctx, deploy.AppID)
		s.finalizeCancelled(ctx, deploy, app)
		return nil
	}

	app, err := s.store.Applications().GetByID(ctx, deploy.AppID)
	if err != nil {
		return fmt.Errorf("load application: %w", err)
	}

	jobCtx, cancel := context.WithTimeout(ctx, deployJobTimeout)
	defer cancel()
	stopPoll := s.startCancelPoll(jobCtx, cancel, deployID)
	defer stopPoll()

	s.executeDeploy(jobCtx, app, deploy, forceBuild)
	return nil
}

func isTerminalDeployStatus(status model.DeployStatus) bool {
	switch status {
	case model.DeploySuccess, model.DeployFailed, model.DeployCancelled:
		return true
	default:
		return false
	}
}

// startCancelPoll watches cancel_requested and cancels jobCtx when set.
func (s *DeployService) startCancelPoll(jobCtx context.Context, cancel context.CancelFunc, deployID uuid.UUID) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				deploy, err := s.store.Deployments().GetByID(context.Background(), deployID)
				if err != nil {
					continue
				}
				if deploy.CancelRequested {
					cancel()
					return
				}
			}
		}
	}()
	return func() { close(done) }
}

// executeDeploy runs the deploy via orchestrator and updates status.
func (s *DeployService) executeDeploy(ctx context.Context, app *model.Application, deploy *model.Deployment, forceBuild bool) {
	now := time.Now()
	deploy.StartedAt = &now

	if app.SourceType == model.SourceGit {
		skipBuild := false

		if !forceBuild && !app.NoCache && deploy.CommitSHA != "" {
			lastDeploy, err := s.store.Deployments().GetLatestByApp(ctx, app.ID)
			if err == nil && lastDeploy.Status == model.DeploySuccess && lastDeploy.Image != "" &&
				lastDeploy.CommitSHA != "" && lastDeploy.CommitSHA == deploy.CommitSHA {
				skipBuild = true
				s.logger.Info("skipping build — same commit, reusing image",
					slog.String("app", app.Name),
					slog.String("commit", deploy.CommitSHA),
					slog.String("image", lastDeploy.Image),
				)
				deploy.BuildLog = fmt.Sprintf("Build skipped — commit %s unchanged, reusing image.\nUse 'Force Build' to rebuild from scratch.", deploy.CommitSHA[:minLen(7, len(deploy.CommitSHA))])
				deploy.Image = lastDeploy.Image
				app.DockerImage = lastDeploy.Image
				s.updateDeploy(ctx, deploy)
			}
		}

		if !skipBuild {
			deploy.Status = model.DeployBuilding
			s.updateDeploy(ctx, deploy)

			s.logger.Info("building from source", slog.String("app", app.Name))
			if err := s.buildSvc.Build(ctx, app, deploy); err != nil {
				// A genuine cancellation (ctx cancelled, not a timeout) must restore
				// the app's prior Running/Idle status instead of marking it Error.
				// finalizeCancelled checks the live K8s status to do this correctly.
				if ctx.Err() != nil && ctx.Err() != context.DeadlineExceeded {
					s.finalizeCancelledFromContext(ctx, deploy, app)
					return
				}
				// Race: Cancel() may have deleted the build job (causing this
				// non-context error) before the poll fired cancel(). Honour the
				// DB cancel flag so the user cancellation is not mislabelled failed.
				if ctx.Err() == nil && s.cancelRequested(deploy.ID) {
					s.finalizeCancelled(context.Background(), deploy, app)
					return
				}
				now := time.Now()
				deploy.FinishedAt = &now
				deploy.Status = model.DeployFailed
				if ctx.Err() == context.DeadlineExceeded {
					deploy.BuildLog += "\n\n--- Build timed out (30 min limit) ---"
					s.notifyDeploy(app, model.EventBuildTimeout, "build timed out (30 min limit)")
				} else {
					s.notifyDeploy(app, model.EventDeployFailed, fmt.Sprintf("build failed: %v", err))
				}
				s.updateDeploy(context.Background(), deploy)
				s.setAppStatus(context.Background(), app.ID, model.AppStatusError)
				s.logger.Error("build failed", slog.Any("error", err), slog.String("app", app.Name))
				return
			}
			if ctx.Err() != nil {
				s.finalizeCancelledFromContext(ctx, deploy, app)
				return
			}
			updatedApp, err := s.store.Applications().GetByID(ctx, app.ID)
			if err != nil {
				s.logger.Error("failed to reload app after build", slog.Any("error", err))
				now := time.Now()
				deploy.Status = model.DeployFailed
				deploy.FinishedAt = &now
				deploy.BuildLog += "\n\n--- Failed to reload app after build ---"
				s.updateDeploy(context.Background(), deploy)
				s.setAppStatus(context.Background(), app.ID, model.AppStatusError)
				s.notifyDeploy(app, model.EventDeployFailed, "failed to reload app after build")
				return
			}
			app = updatedApp
		}
	}

	deploy.Status = model.DeployDeploying
	s.updateDeploy(ctx, deploy)

	mergedEnv := make(map[string]string)
	project, projErr := s.store.Projects().GetByID(ctx, app.ProjectID)
	if projErr == nil && project.EnvVars != nil {
		for k, v := range project.EnvVars {
			mergedEnv[k] = v
		}
	}
	for k, v := range app.EnvVars {
		mergedEnv[k] = v
	}

	pullSecret, err := s.registryAuth.DockerConfigJSONForApp(ctx, app)
	if err != nil {
		now := time.Now()
		deploy.Status = model.DeployFailed
		deploy.FinishedAt = &now
		s.updateDeploy(ctx, deploy)
		s.setAppStatus(ctx, app.ID, model.AppStatusError)
		s.logger.Error("failed to resolve registry credentials", slog.Any("error", err), slog.String("app", app.Name))
		s.notifyDeploy(app, model.EventDeployFailed, fmt.Sprintf("registry auth failed: %v", err))
		return
	}

	target, terr := targetForApp(s.targets, app)
	if terr != nil {
		now := time.Now()
		deploy.Status = model.DeployFailed
		deploy.FinishedAt = &now
		s.updateDeploy(ctx, deploy)
		s.setAppStatus(ctx, app.ID, model.AppStatusError)
		s.logger.Error("failed to resolve deploy target", slog.Any("error", terr), slog.String("app", app.Name))
		s.notifyDeploy(app, model.EventDeployFailed, fmt.Sprintf("deploy target error: %v", terr))
		return
	}

	err = target.Deploy(ctx, app, orchestrator.DeployOpts{
		Image:                  app.DockerImage,
		Replicas:               app.Replicas,
		EnvVars:                mergedEnv,
		Ports:                  app.Ports,
		CPULimit:               app.CPULimit,
		MemLimit:               app.MemLimit,
		CPURequest:             app.CPURequest,
		MemRequest:             app.MemRequest,
		HealthCheck:            app.HealthCheck,
		Volumes:                app.Volumes,
		DeployStrategy:         app.DeployStrategy,
		DeployStrategyConfig:   app.DeployStrategyConfig,
		TerminationGracePeriod: app.TerminationGracePeriod,
		NodePool:               app.NodePool,
		ImagePullSecret:        pullSecret,
	})

	now = time.Now()
	deploy.FinishedAt = &now

	if err != nil {
		if ctx.Err() != nil {
			s.finalizeCancelledFromContext(ctx, deploy, app)
			return
		}
		if s.cancelRequested(deploy.ID) {
			s.finalizeCancelled(context.Background(), deploy, app)
			return
		}
		deploy.Status = model.DeployFailed
		s.updateDeploy(ctx, deploy)
		s.setAppStatus(ctx, app.ID, model.AppStatusError)
		s.logger.Error("deploy failed", slog.Any("error", err), slog.String("app", app.Name))
		s.notifyDeploy(app, model.EventDeployFailed, fmt.Sprintf("deploy failed: %v", err))
		return
	}

	deploy.Status = model.DeploySuccess
	deploy.Image = app.DockerImage
	s.updateDeploy(ctx, deploy)
	s.setAppStatus(ctx, app.ID, model.AppStatusRunning)
	_ = s.store.Applications().Update(ctx, app)

	if app.Autoscaling != nil && app.Autoscaling.Enabled {
		if err := target.ConfigureHPA(ctx, app, *app.Autoscaling); err != nil {
			s.logger.Error("failed to restore HPA after deploy — autoscaling inactive until manually re-saved",
				slog.String("app", app.Name), slog.Any("error", err))
			s.notifyDeploy(app, model.EventDeploySuccess,
				fmt.Sprintf("%s deployed successfully, but autoscaling (HPA) failed to restore — re-save autoscaling settings to fix", app.Name))
			return
		}
	}

	s.logger.Info("deploy succeeded", slog.String("app", app.Name), slog.String("deploy", deploy.ID.String()))
	s.notifyDeploy(app, model.EventDeploySuccess, fmt.Sprintf("%s deployed successfully", app.Name))

	go s.cleanupCompletedPods(app)
}

func (s *DeployService) finalizeCancelledFromContext(ctx context.Context, deploy *model.Deployment, app *model.Application) {
	if ctx.Err() == nil {
		return
	}
	s.finalizeCancelled(context.Background(), deploy, app)
}

func (s *DeployService) cancelRequested(deployID uuid.UUID) bool {
	d, err := s.store.Deployments().GetByID(context.Background(), deployID)
	return err == nil && d != nil && d.CancelRequested
}

func (s *DeployService) finalizeCancelled(ctx context.Context, deploy *model.Deployment, app *model.Application) {
	now := time.Now()
	deploy.Status = model.DeployCancelled
	deploy.FinishedAt = &now
	if deploy.BuildLog != "" {
		deploy.BuildLog += "\n\n--- Cancelled by user ---"
	} else {
		deploy.BuildLog = "Cancelled by user"
	}
	s.updateDeploy(ctx, deploy)

	if app != nil {
		s.notifyDeploy(app, model.EventDeployCancelled, fmt.Sprintf("%s deploy cancelled by user", app.Name))
		target, targetErr := targetForApp(s.targets, app)
		if targetErr == nil {
			status, statusErr := target.GetStatus(ctx, app)
			if statusErr == nil && status.Phase == "running" {
				s.setAppStatus(ctx, deploy.AppID, model.AppStatusRunning)
				return
			}
		}
	}
	s.setAppStatus(ctx, deploy.AppID, model.AppStatusIdle)
}

func (s *DeployService) notifyDeploy(app *model.Application, event model.NotifyEvent, detail string) {
	if s.notifSvc == nil {
		return
	}
	project, err := s.store.Projects().GetByID(context.Background(), app.ProjectID)
	if err != nil {
		s.logger.Warn("failed to resolve org for deploy notification", slog.Any("error", err))
		return
	}
	title := fmt.Sprintf("%s: %s", app.Name, string(event))
	s.notifSvc.NotifyAsync(project.OrgID, event, title, detail)
}

func (s *DeployService) updateDeploy(ctx context.Context, deploy *model.Deployment) {
	if err := s.store.Deployments().Update(ctx, deploy); err != nil {
		s.logger.Error("failed to update deployment",
			slog.String("deploy_id", deploy.ID.String()),
			slog.String("status", string(deploy.Status)),
			slog.Any("error", err),
		)
	}
}

func (s *DeployService) cleanupCompletedPods(app *model.Application) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	target, err := targetForApp(s.targets, app)
	if err != nil {
		return
	}
	pods, err := target.GetPods(ctx, app)
	if err != nil {
		return
	}
	for _, pod := range pods {
		if pod.Phase == "Succeeded" || pod.Phase == "Failed" {
			if err := target.DeletePod(ctx, pod.Name, app); err != nil {
				s.logger.Warn("failed to cleanup completed pod", slog.String("pod", pod.Name), slog.Any("error", err))
			} else {
				s.logger.Info("cleaned up completed pod", slog.String("pod", pod.Name))
			}
		}
	}
}

func (s *DeployService) setAppStatus(ctx context.Context, appID uuid.UUID, status model.AppStatus) {
	if err := s.store.Applications().UpdateStatus(ctx, appID, status); err != nil {
		s.logger.Error("failed to update app status",
			slog.String("app_id", appID.String()),
			slog.String("target_status", string(status)),
			slog.Any("error", err),
		)
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Cancel requests cancellation of a running or queued deployment.
func (s *DeployService) Cancel(ctx context.Context, deployID uuid.UUID) error {
	deploy, err := s.store.Deployments().GetByID(ctx, deployID)
	if err != nil {
		return err
	}

	if deploy.Status != model.DeployQueued && deploy.Status != model.DeployBuilding && deploy.Status != model.DeployDeploying {
		return fmt.Errorf("deployment is not in progress (status: %s)", deploy.Status)
	}

	deploy.CancelRequested = true
	if err := s.store.Deployments().Update(ctx, deploy); err != nil {
		return fmt.Errorf("set cancel_requested: %w", err)
	}

	app, err := s.store.Applications().GetByID(ctx, deploy.AppID)
	if err == nil {
		s.cleanupBuildJobs(ctx, app)
	}

	if deploy.Status == model.DeployQueued {
		s.finalizeCancelled(ctx, deploy, app)
	}

	s.logger.Info("deployment cancel requested", slog.String("deploy", deployID.String()))
	return nil
}

func (s *DeployService) cleanupBuildJobs(ctx context.Context, app *model.Application) {
	builder, err := targetBuilder(s.targets, app)
	if err != nil {
		return
	}
	_ = builder.CancelBuild(ctx, app)
}

func (s *DeployService) GetByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	return s.store.Deployments().GetByID(ctx, id)
}

func (s *DeployService) List(ctx context.Context, appID uuid.UUID, params store.ListParams) ([]model.Deployment, int, error) {
	return s.store.Deployments().ListByApp(ctx, appID, params)
}

func (s *DeployService) ListAll(ctx context.Context, params store.ListParams, filter store.DeploymentListFilter) ([]model.Deployment, int, error) {
	return s.store.Deployments().ListAll(ctx, params, filter)
}

func (s *DeployService) Rollback(ctx context.Context, deployID uuid.UUID, triggeredBy *uuid.UUID) (*model.Deployment, error) {
	prev, err := s.store.Deployments().GetByID(ctx, deployID)
	if err != nil {
		return nil, err
	}

	app, err := s.store.Applications().GetByID(ctx, prev.AppID)
	if err != nil {
		return nil, err
	}

	target, err := targetForApp(s.targets, app)
	if err != nil {
		return nil, err
	}
	if err := target.Rollback(ctx, app, 0); err != nil {
		return nil, err
	}

	now := time.Now()
	deploy := &model.Deployment{
		AppID:       app.ID,
		Status:      model.DeploySuccess,
		Image:       prev.Image,
		CommitSHA:   prev.CommitSHA,
		TriggerType: "rollback",
		TriggeredBy: triggeredBy,
		StartedAt:   &now,
		FinishedAt:  &now,
	}

	if err := s.store.Deployments().Create(ctx, deploy); err != nil {
		return nil, err
	}

	s.setAppStatus(ctx, app.ID, model.AppStatusRunning)
	s.logger.Info("rollback succeeded", slog.String("app", app.Name), slog.String("to_deploy", deployID.String()))

	return deploy, nil
}
