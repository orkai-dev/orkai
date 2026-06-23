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
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/workers"
)

// workerTeardownTimeout bounds a detached best-effort `wrangler delete` so a
// stuck build pod can't leak a goroutine indefinitely.
const workerTeardownTimeout = 20 * time.Minute

type WorkerService struct {
	store     store.Store
	targets   *orchestrator.TargetRegistry
	providers *providers.Registry
	logger    *slog.Logger
	notifSvc  *NotificationService

	// teardownWG tracks in-flight detached script teardowns so they can be
	// awaited (graceful shutdown, tests).
	teardownWG sync.WaitGroup
}

func NewWorkerService(s store.Store, targets *orchestrator.TargetRegistry, prov *providers.Registry, logger *slog.Logger, notifSvc *NotificationService) *WorkerService {
	return &WorkerService{store: s, targets: targets, providers: prov, logger: logger, notifSvc: notifSvc}
}

// WaitForTeardowns blocks until all detached script teardowns started by Delete
// have finished. Intended for graceful shutdown and deterministic tests.
func (s *WorkerService) WaitForTeardowns() { s.teardownWG.Wait() }

type CreateWorkerInput struct {
	ProjectID      uuid.UUID         `json:"project_id" binding:"required"`
	Name           string            `json:"name" binding:"required,min=1,max=63"`
	Description    string            `json:"description"`
	GitRepo        string            `json:"git_repo" binding:"required"`
	GitBranch      string            `json:"git_branch"`
	GitProviderID  *uuid.UUID        `json:"git_provider_id"`
	RootDirectory  string            `json:"root_directory"`
	WranglerConfig string            `json:"wrangler_config"`
	PackageManager string            `json:"package_manager"`
	InstallCommand string            `json:"install_command"`
	BuildCommand   string            `json:"build_command"`
	DeployCommand  string            `json:"deploy_command"`
	BuildEnvVars   map[string]string `json:"build_env_vars"`
	CloudAccountID *uuid.UUID        `json:"cloud_account_id"`
}

func (s *WorkerService) Create(ctx context.Context, input CreateWorkerInput) (*model.Worker, error) {
	if !safeNameRe.MatchString(input.Name) {
		return nil, fmt.Errorf("worker name must start with alphanumeric and contain only letters, numbers, hyphens, and underscores")
	}

	project, err := s.store.Projects().GetByID(ctx, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	if input.CloudAccountID != nil {
		if err := s.validateResource(ctx, project.OrgID, *input.CloudAccountID, model.ResourceCloudAccount); err != nil {
			return nil, err
		}
		if err := validateWorkerCloudAccount(ctx, s.store, *input.CloudAccountID); err != nil {
			return nil, err
		}
	}
	if input.GitProviderID != nil {
		if err := s.validateResource(ctx, project.OrgID, *input.GitProviderID, model.ResourceGitProvider); err != nil {
			return nil, err
		}
	}
	if err := validatePackageManager(input.PackageManager); err != nil {
		return nil, err
	}

	buildEnv := input.BuildEnvVars
	if buildEnv == nil {
		buildEnv = map[string]string{}
	}

	worker := &model.Worker{
		ProjectID:      input.ProjectID,
		Name:           input.Name,
		Description:    input.Description,
		GitRepo:        input.GitRepo,
		GitBranch:      input.GitBranch,
		GitProviderID:  input.GitProviderID,
		RootDirectory:  input.RootDirectory,
		WranglerConfig: strings.TrimSpace(input.WranglerConfig),
		PackageManager: input.PackageManager,
		InstallCommand: input.InstallCommand,
		BuildCommand:   input.BuildCommand,
		DeployCommand:  input.DeployCommand,
		BuildEnvVars:   buildEnv,
		CloudAccountID: input.CloudAccountID,
		Runtime:        &model.WorkerRuntime{},
		Status:         model.WorkerStatusIdle,
	}
	if worker.GitBranch == "" {
		worker.GitBranch = "main"
	}
	if worker.RootDirectory == "" {
		worker.RootDirectory = "."
	}
	if worker.WranglerConfig == "" {
		worker.WranglerConfig = "wrangler.toml"
	}
	if worker.PackageManager == "" {
		worker.PackageManager = "auto"
	}

	if dup, err := s.store.Workers().ExistsByName(ctx, input.ProjectID, worker.Name); err != nil {
		return nil, fmt.Errorf("cannot verify name uniqueness: %w", err)
	} else if dup {
		return nil, fmt.Errorf("a worker named %q already exists in this project", worker.Name)
	}

	if err := s.store.Workers().Create(ctx, worker); err != nil {
		if strings.Contains(err.Error(), "idx_workers_project_id_name_active") {
			return nil, fmt.Errorf("a worker named %q already exists in this project", worker.Name)
		}
		return nil, err
	}
	s.logger.Info("worker created", slog.String("name", worker.Name), slog.String("id", worker.ID.String()))
	return worker, nil
}

func (s *WorkerService) GetByID(ctx context.Context, id uuid.UUID) (*model.Worker, error) {
	return s.store.Workers().GetByID(ctx, id)
}

func (s *WorkerService) List(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Worker, int, error) {
	return s.store.Workers().ListByProject(ctx, projectID, params)
}

func (s *WorkerService) ListAll(ctx context.Context, params store.ListParams, filter store.WorkerListFilter) ([]model.Worker, int, error) {
	return s.store.Workers().ListAll(ctx, params, filter)
}

type UpdateWorkerInput struct {
	Description    *string            `json:"description"`
	GitRepo        *string            `json:"git_repo"`
	GitBranch      *string            `json:"git_branch"`
	GitProviderID  *uuid.UUID         `json:"git_provider_id"`
	RootDirectory  *string            `json:"root_directory"`
	WranglerConfig *string            `json:"wrangler_config"`
	PackageManager *string            `json:"package_manager"`
	InstallCommand *string            `json:"install_command"`
	BuildCommand   *string            `json:"build_command"`
	DeployCommand  *string            `json:"deploy_command"`
	BuildEnvVars   *map[string]string `json:"build_env_vars"`
	CloudAccountID *uuid.UUID         `json:"cloud_account_id"`
}

func (s *WorkerService) Update(ctx context.Context, id uuid.UUID, input UpdateWorkerInput) (*model.Worker, error) {
	worker, err := s.store.Workers().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var orgID *uuid.UUID
	resolveOrg := func() (uuid.UUID, error) {
		if orgID != nil {
			return *orgID, nil
		}
		project, perr := s.store.Projects().GetByID(ctx, worker.ProjectID)
		if perr != nil {
			return uuid.Nil, fmt.Errorf("project not found: %w", perr)
		}
		orgID = &project.OrgID
		return project.OrgID, nil
	}

	if input.Description != nil {
		worker.Description = *input.Description
	}
	if input.GitRepo != nil {
		worker.GitRepo = *input.GitRepo
	}
	if input.GitBranch != nil {
		branch := strings.TrimSpace(*input.GitBranch)
		if branch == "" {
			branch = "main"
		}
		worker.GitBranch = branch
	}
	if input.GitProviderID != nil {
		org, oerr := resolveOrg()
		if oerr != nil {
			return nil, oerr
		}
		if verr := s.validateResource(ctx, org, *input.GitProviderID, model.ResourceGitProvider); verr != nil {
			return nil, verr
		}
		worker.GitProviderID = input.GitProviderID
	}
	if input.RootDirectory != nil {
		root := strings.TrimSpace(*input.RootDirectory)
		if root == "" {
			root = "."
		}
		worker.RootDirectory = root
	}
	if input.WranglerConfig != nil {
		cfg := strings.TrimSpace(*input.WranglerConfig)
		if cfg == "" {
			cfg = "wrangler.toml"
		}
		worker.WranglerConfig = cfg
	}
	if input.PackageManager != nil {
		if err := validatePackageManager(*input.PackageManager); err != nil {
			return nil, err
		}
		pm := strings.TrimSpace(*input.PackageManager)
		if pm == "" {
			pm = "auto"
		}
		worker.PackageManager = pm
	}
	if input.InstallCommand != nil {
		worker.InstallCommand = *input.InstallCommand
	}
	if input.BuildCommand != nil {
		worker.BuildCommand = *input.BuildCommand
	}
	if input.DeployCommand != nil {
		worker.DeployCommand = *input.DeployCommand
	}
	if input.BuildEnvVars != nil {
		worker.BuildEnvVars = *input.BuildEnvVars
	}

	cloudTargetChanging := false
	if input.CloudAccountID != nil {
		changing := worker.CloudAccountID == nil || *input.CloudAccountID != *worker.CloudAccountID
		org, oerr := resolveOrg()
		if oerr != nil {
			return nil, oerr
		}
		if verr := s.validateResource(ctx, org, *input.CloudAccountID, model.ResourceCloudAccount); verr != nil {
			return nil, verr
		}
		if verr := validateWorkerCloudAccount(ctx, s.store, *input.CloudAccountID); verr != nil {
			return nil, verr
		}
		worker.CloudAccountID = input.CloudAccountID
		cloudTargetChanging = changing
	}

	if cloudTargetChanging {
		applied, err := s.store.Workers().UpdateSettingsIfNotDeploying(ctx, worker)
		if err != nil {
			return nil, err
		}
		if !applied {
			return nil, fmt.Errorf("cannot change cloud account while a deploy is in progress")
		}
		return worker, nil
	}
	if err := s.store.Workers().UpdateSettings(ctx, worker); err != nil {
		return nil, err
	}
	return worker, nil
}

func (s *WorkerService) Delete(ctx context.Context, id uuid.UUID) error {
	worker, err := s.store.Workers().GetByID(ctx, id)
	if err != nil {
		return err
	}
	if worker.Status == model.WorkerStatusDeploying {
		return fmt.Errorf("worker is currently deploying, wait for it to finish before deleting")
	}

	deleted, err := s.store.Workers().DeleteIfNotDeploying(ctx, id)
	if err != nil {
		return err
	}
	if deleted == nil {
		return fmt.Errorf("worker is currently deploying, wait for it to finish before deleting")
	}

	// Best-effort teardown of the Cloudflare script via an in-cluster
	// `wrangler delete` pod. The DB row is already gone, so we detach the
	// teardown from the request lifecycle.
	if deleted.Runtime != nil && deleted.Runtime.ScriptName != "" && deleted.CloudAccountID != nil {
		s.teardownScriptAsync(ctx, deleted)
	}

	notifyProjectResourceDeleted(s.notifSvc, s.store, s.logger, deleted.ProjectID, model.EventWorkerDeleted,
		deleted.Name, fmt.Sprintf("Worker %q was deleted", deleted.Name))
	return nil
}

// teardownScriptAsync runs `wrangler delete` in a build pod, detached from the
// caller's context but bounded by workerTeardownTimeout. Failures are logged for
// manual follow-up; the DB record is already removed.
func (s *WorkerService) teardownScriptAsync(ctx context.Context, worker *model.Worker) {
	bg := context.WithoutCancel(ctx)
	s.teardownWG.Add(1)
	go func() {
		defer s.teardownWG.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("worker delete: script teardown panicked — clean up Cloudflare manually",
					slog.String("worker", worker.Name), slog.Any("panic", r))
			}
		}()

		tctx, cancel := context.WithTimeout(bg, workerTeardownTimeout)
		defer cancel()

		builder, berr := orchestrator.AsCapability[orchestrator.WorkerBuilder](s.targets.Default(), orchestrator.CapWorkerBuild)
		if berr != nil {
			s.logger.Warn("worker delete: builder unavailable — Cloudflare script not torn down",
				slog.String("worker", worker.Name), slog.Any("error", berr))
			return
		}

		cfg, cerr := resolveCloudConfig(tctx, s.store, worker.CloudAccountID)
		if cerr != nil {
			s.logger.Warn("worker delete: cannot resolve cloud account — clean up Cloudflare manually",
				slog.String("worker", worker.Name), slog.Any("error", cerr))
			return
		}
		apiToken, accountID, derr := workers.DeployCredentials(cfg)
		if derr != nil {
			s.logger.Warn("worker delete: invalid cloud credentials — clean up Cloudflare manually",
				slog.String("worker", worker.Name), slog.Any("error", derr))
			return
		}
		token, _ := resolveWorkerGitToken(tctx, s.providers, s.store, worker)

		if _, err := builder.DeleteWorker(tctx, orchestrator.WorkerDeleteOpts{
			GitRepo:        worker.GitRepo,
			GitBranch:      worker.GitBranch,
			GitToken:       token,
			WorkerID:       worker.ID.String(),
			RootDirectory:  worker.RootDirectory,
			WranglerConfig: worker.WranglerConfig,
			PackageManager: worker.PackageManager,
			InstallCommand: worker.InstallCommand,
			ScriptName:     worker.Runtime.ScriptName,
			CFAPIToken:     apiToken,
			CFAccountID:    accountID,
		}); err != nil {
			s.logger.Warn("worker delete: script teardown failed — DB record removed, clean up Cloudflare manually",
				slog.String("worker", worker.Name),
				slog.String("script", worker.Runtime.ScriptName),
				slog.Any("error", err))
		}
	}()
}

// ListDeployments returns the deployment history for a worker.
func (s *WorkerService) ListDeployments(ctx context.Context, workerID uuid.UUID, params store.ListParams) ([]model.WorkerDeployment, int, error) {
	return s.store.WorkerDeployments().ListByWorker(ctx, workerID, params)
}

func (s *WorkerService) GetDeployment(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error) {
	return s.store.WorkerDeployments().GetByID(ctx, id)
}

// validateResource ensures a referenced shared resource exists, has the expected
// type, and belongs to the given org.
func (s *WorkerService) validateResource(ctx context.Context, orgID, id uuid.UUID, want model.ResourceType) error {
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

// validateWorkerCloudAccount requires the linked cloud account to be a
// Cloudflare account, since Workers deploy via wrangler against Cloudflare.
func validateWorkerCloudAccount(ctx context.Context, st store.Store, cloudAccountID uuid.UUID) error {
	res, err := st.SharedResources().GetByID(ctx, cloudAccountID)
	if err != nil {
		return fmt.Errorf("cloud account not found: %w", err)
	}
	if res.Provider != "cloudflare" {
		return fmt.Errorf("workers require a Cloudflare cloud account, but %q was selected", res.Provider)
	}
	return nil
}

// resolveWorkerGitToken resolves a git clone token for a worker: prefer a fresh
// GitHub App installation token, then fall back to a linked provider's stored
// token. Mirrors PagePublishService.resolveGitToken.
func resolveWorkerGitToken(ctx context.Context, prov *providers.Registry, st store.Store, worker *model.Worker) (string, error) {
	var reasons []string

	if gh, err := prov.Git("github"); err != nil {
		reasons = append(reasons, "github app unavailable: "+err.Error())
	} else if token, terr := gh.CloneToken(ctx, worker.GitRepo, nil); terr != nil {
		reasons = append(reasons, "github app installation token: "+terr.Error())
	} else if token != "" {
		return token, nil
	} else {
		reasons = append(reasons, "github app: no installation token for this repository")
	}

	if worker.GitProviderID != nil {
		resource, err := st.SharedResources().GetByID(ctx, *worker.GitProviderID)
		if err != nil {
			reasons = append(reasons, "git provider resource: "+err.Error())
		} else {
			var cfg struct {
				Token string `json:"token"`
			}
			if jerr := json.Unmarshal(resource.Config, &cfg); jerr != nil {
				reasons = append(reasons, "git provider config parse: "+jerr.Error())
			} else if cfg.Token == "" {
				reasons = append(reasons, "git provider resource has no token configured")
			} else {
				return cfg.Token, nil
			}
		}
	} else {
		reasons = append(reasons, "no git provider linked to the worker")
	}

	return "", fmt.Errorf("no git token available for worker %s: %s", worker.Name, strings.Join(reasons, "; "))
}
