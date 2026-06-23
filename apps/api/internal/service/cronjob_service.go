package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type CronJobService struct {
	store    store.Store
	targets  *orchestrator.TargetRegistry
	logger   *slog.Logger
	notifSvc *NotificationService
}

func NewCronJobService(s store.Store, targets *orchestrator.TargetRegistry, logger *slog.Logger, notifSvc *NotificationService) *CronJobService {
	return &CronJobService{store: s, targets: targets, logger: logger, notifSvc: notifSvc}
}

type CreateCronJobInput struct {
	ProjectID             uuid.UUID         `json:"project_id" binding:"required"`
	Name                  string            `json:"name" binding:"required,min=1,max=63"`
	Description           string            `json:"description"`
	CronExpression        string            `json:"cron_expression" binding:"required"`
	Timezone              string            `json:"timezone"`
	Command               string            `json:"command" binding:"required"`
	Image                 string            `json:"image"`
	SourceType            string            `json:"source_type"`
	GitRepo               string            `json:"git_repo"`
	GitBranch             string            `json:"git_branch"`
	EnvVars               map[string]string `json:"env_vars"`
	CPULimit              string            `json:"cpu_limit"`
	MemLimit              string            `json:"mem_limit"`
	Enabled               bool              `json:"enabled"`
	ConcurrencyPolicy     string            `json:"concurrency_policy"`
	RestartPolicy         string            `json:"restart_policy"`
	BackoffLimit          *int              `json:"backoff_limit"`
	ActiveDeadlineSeconds int               `json:"active_deadline_seconds"`
}

func (s *CronJobService) Create(ctx context.Context, input CreateCronJobInput) (*model.CronJob, error) {
	// Look up project namespace
	project, err := s.store.Projects().GetByID(ctx, input.ProjectID)
	if err != nil {
		return nil, err
	}

	cj := &model.CronJob{
		ProjectID:             input.ProjectID,
		Name:                  input.Name,
		Description:           input.Description,
		CronExpression:        input.CronExpression,
		Timezone:              input.Timezone,
		Command:               input.Command,
		Image:                 input.Image,
		SourceType:            model.SourceType(input.SourceType),
		GitRepo:               input.GitRepo,
		GitBranch:             input.GitBranch,
		EnvVars:               input.EnvVars,
		CPULimit:              input.CPULimit,
		MemLimit:              input.MemLimit,
		Enabled:               input.Enabled,
		ConcurrencyPolicy:     input.ConcurrencyPolicy,
		RestartPolicy:         input.RestartPolicy,
		BackoffLimit:          derefIntOrZero(input.BackoffLimit),
		ActiveDeadlineSeconds: input.ActiveDeadlineSeconds,
		Namespace:             project.Namespace,
		Status:                model.CronJobIdle,
	}

	// Defaults
	if cj.Timezone == "" {
		cj.Timezone = "UTC"
	}
	if cj.SourceType == "" {
		cj.SourceType = model.SourceImage
	}
	if cj.SourceType == model.SourceGit {
		return nil, fmt.Errorf("CronJobs do not support git source type — use a pre-built container image instead")
	}
	if cj.Image == "" {
		cj.Image = "busybox:latest"
	}
	if cj.GitBranch == "" {
		cj.GitBranch = "main"
	}
	if cj.CPULimit == "" {
		cj.CPULimit = "500m"
	}
	if cj.MemLimit == "" {
		cj.MemLimit = "512Mi"
	}
	if cj.ConcurrencyPolicy == "" {
		cj.ConcurrencyPolicy = "Forbid"
	}
	if cj.RestartPolicy == "" {
		cj.RestartPolicy = "OnFailure"
	}
	if cj.BackoffLimit == 0 && input.BackoffLimit == nil {
		cj.BackoffLimit = 3
	}
	if cj.EnvVars == nil {
		cj.EnvVars = map[string]string{}
	}

	if err := s.store.CronJobs().Create(ctx, cj); err != nil {
		return nil, err
	}

	// Create K8s CronJob with merged env (project base + cronjob override)
	k8sCJ := s.withMergedEnv(cj, project.EnvVars)
	cron, err := targetCron(s.targets)
	if err != nil {
		_ = s.store.CronJobs().Delete(ctx, cj.ID)
		return nil, fmt.Errorf("cron capability unavailable: %w", err)
	}
	if err := cron.CreateCronJob(ctx, k8sCJ); err != nil {
		s.logger.Error("failed to create K8s CronJob", slog.Any("error", err), slog.String("name", cj.Name))
		_ = s.store.CronJobs().Delete(ctx, cj.ID)
		return nil, fmt.Errorf("failed to create K8s CronJob: %w", err)
	}

	s.logger.Info("cronjob created", slog.String("name", cj.Name), slog.String("schedule", cj.CronExpression))
	return cj, nil
}

type UpdateCronJobInput struct {
	CronExpression        *string           `json:"cron_expression"`
	Timezone              *string           `json:"timezone"`
	Command               *string           `json:"command"`
	Image                 *string           `json:"image"`
	EnvVars               map[string]string `json:"env_vars"`
	CPULimit              *string           `json:"cpu_limit"`
	MemLimit              *string           `json:"mem_limit"`
	Enabled               *bool             `json:"enabled"`
	ConcurrencyPolicy     *string           `json:"concurrency_policy"`
	RestartPolicy         *string           `json:"restart_policy"`
	BackoffLimit          *int              `json:"backoff_limit"`
	ActiveDeadlineSeconds *int              `json:"active_deadline_seconds"`
	Description           *string           `json:"description"`
}

func (s *CronJobService) Update(ctx context.Context, id uuid.UUID, input UpdateCronJobInput) (*model.CronJob, error) {
	cj, err := s.store.CronJobs().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.CronExpression != nil {
		cj.CronExpression = *input.CronExpression
	}
	if input.Timezone != nil {
		cj.Timezone = *input.Timezone
	}
	if input.Command != nil {
		cj.Command = *input.Command
	}
	if input.Image != nil {
		cj.Image = *input.Image
	}
	if input.EnvVars != nil {
		cj.EnvVars = input.EnvVars
	}
	if input.CPULimit != nil {
		cj.CPULimit = *input.CPULimit
	}
	if input.MemLimit != nil {
		cj.MemLimit = *input.MemLimit
	}
	if input.Enabled != nil {
		cj.Enabled = *input.Enabled
	}
	if input.ConcurrencyPolicy != nil {
		cj.ConcurrencyPolicy = *input.ConcurrencyPolicy
	}
	if input.RestartPolicy != nil {
		cj.RestartPolicy = *input.RestartPolicy
	}
	if input.BackoffLimit != nil {
		cj.BackoffLimit = *input.BackoffLimit
	}
	if input.ActiveDeadlineSeconds != nil {
		cj.ActiveDeadlineSeconds = *input.ActiveDeadlineSeconds
	}
	if input.Description != nil {
		cj.Description = *input.Description
	}

	if err := s.store.CronJobs().Update(ctx, cj); err != nil {
		return nil, err
	}

	// Update K8s CronJob with merged env
	project, _ := s.store.Projects().GetByID(ctx, cj.ProjectID)
	k8sCJ := s.withMergedEnv(cj, projectEnvOrNil(project))
	cron, err := targetCron(s.targets)
	if err != nil {
		return nil, fmt.Errorf("cron capability unavailable: %w", err)
	}
	if err := cron.UpdateCronJob(ctx, k8sCJ); err != nil {
		s.logger.Error("failed to update K8s CronJob", slog.Any("error", err), slog.String("name", cj.Name))
		return nil, fmt.Errorf("failed to update K8s CronJob: %w", err)
	}

	return cj, nil
}

func (s *CronJobService) GetByID(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
	return s.store.CronJobs().GetByID(ctx, id)
}

func (s *CronJobService) List(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.CronJob, int, error) {
	return s.store.CronJobs().ListByProject(ctx, projectID, params)
}

func (s *CronJobService) Delete(ctx context.Context, id uuid.UUID) error {
	cj, err := s.store.CronJobs().GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete from K8s
	cron, err := targetCron(s.targets)
	if err != nil {
		return fmt.Errorf("cron capability unavailable: %w", err)
	}
	if err := cron.DeleteCronJob(ctx, cj); err != nil {
		s.logger.Error("failed to delete K8s CronJob", slog.Any("error", err), slog.String("name", cj.Name))
		return fmt.Errorf("failed to delete K8s CronJob: %w — delete manually before retrying", err)
	}

	if err := s.store.CronJobs().Delete(ctx, id); err != nil {
		return err
	}

	notifyProjectResourceDeleted(s.notifSvc, s.store, s.logger, cj.ProjectID, model.EventCronJobDeleted,
		cj.Name, fmt.Sprintf("Cron job %q was deleted", cj.Name))
	return nil
}

func (s *CronJobService) Trigger(ctx context.Context, id uuid.UUID) (*model.CronJobRun, error) {
	cj, err := s.store.CronJobs().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	cron, err := targetCron(s.targets)
	if err != nil {
		return nil, err
	}
	jobName, err := cron.TriggerCronJob(ctx, cj)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cj.LastRunAt = &now
	cj.Status = model.CronJobRunning
	_ = s.store.CronJobs().Update(ctx, cj)

	run := &model.CronJobRun{
		CronJobID:   cj.ID,
		Status:      model.CronJobRunRunning,
		StartedAt:   now,
		TriggerType: "manual",
		Logs:        "Job created: " + jobName,
	}
	if err := s.store.CronJobRuns().Create(ctx, run); err != nil {
		return nil, err
	}

	s.logger.Info("cronjob triggered", slog.String("name", cj.Name), slog.String("job", jobName))

	// Watch job completion in background
	go s.watchJobCompletion(cj, run, jobName)

	return run, nil
}

func (s *CronJobService) watchJobCompletion(cj *model.CronJob, run *model.CronJobRun, jobName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timed out — mark as failed
			now := time.Now()
			run.Status = model.CronJobRunFailed
			run.FinishedAt = &now
			run.Logs += "\nTimed out waiting for completion"
			_ = s.store.CronJobRuns().Update(context.Background(), run)
			cj.Status = model.CronJobIdle
			_ = s.store.CronJobs().Update(context.Background(), cj)
			return
		case <-ticker.C:
			cron, cerr := targetCron(s.targets)
			if cerr != nil {
				continue
			}
			status, err := cron.GetJobStatus(ctx, cj, jobName)
			if err != nil {
				continue
			}
			if status == "succeeded" || status == "failed" {
				now := time.Now()
				run.FinishedAt = &now
				if status == "succeeded" {
					run.Status = model.CronJobRunSucceeded
				} else {
					run.Status = model.CronJobRunFailed
				}
				_ = s.store.CronJobRuns().Update(context.Background(), run)
				cj.Status = model.CronJobIdle
				_ = s.store.CronJobs().Update(context.Background(), cj)
				return
			}
		}
	}
}

func (s *CronJobService) ListRuns(ctx context.Context, cronJobID uuid.UUID, params store.ListParams) ([]model.CronJobRun, int, error) {
	return s.store.CronJobRuns().ListByCronJob(ctx, cronJobID, params)
}

// withMergedEnv returns a shallow copy of cj with project env merged in (project as base, cj overrides).
// The original cj is not modified — safe for DB persistence.
func (s *CronJobService) withMergedEnv(cj *model.CronJob, projectEnv map[string]string) *model.CronJob {
	if len(projectEnv) == 0 {
		return cj
	}
	tmp := *cj
	merged := make(map[string]string)
	for k, v := range projectEnv {
		merged[k] = v
	}
	for k, v := range cj.EnvVars {
		merged[k] = v
	}
	tmp.EnvVars = merged
	return &tmp
}

func projectEnvOrNil(p *model.Project) map[string]string {
	if p == nil {
		return nil
	}
	return p.EnvVars
}

func derefIntOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
