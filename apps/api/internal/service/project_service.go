package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type ProjectService struct {
	store    store.Store
	targets  *orchestrator.TargetRegistry
	logger   *slog.Logger
	notifSvc *NotificationService
}

func NewProjectService(s store.Store, targets *orchestrator.TargetRegistry, logger *slog.Logger, notifSvc *NotificationService) *ProjectService {
	return &ProjectService{store: s, targets: targets, logger: logger, notifSvc: notifSvc}
}

type CreateProjectInput struct {
	Name        string            `json:"name" binding:"required,min=1,max=63"`
	Description string            `json:"description"`
	Environment model.Environment `json:"environment" binding:"required,oneof=prod testing sandbox qa poc development"`
	TeamID      uuid.UUID         `json:"team_id" binding:"required"`
}

type UpdateProjectInput struct {
	Name                 *string                    `json:"name" binding:"omitempty,min=1,max=63"`
	Description          *string                    `json:"description"`
	Environment          *model.Environment         `json:"environment"`
	EnvVars              map[string]string          `json:"env_vars"`
	ResourceQuota        *model.ResourceQuotaConfig `json:"resource_quota"`
	NetworkPolicyEnabled *bool                      `json:"network_policy_enabled"`
}

// generateNamespace produces a valid K8s namespace from the project name and ID.
func generateNamespace(name string, id uuid.UUID) string {
	// Check if name is ASCII-only
	isASCII := true
	for _, r := range name {
		if r > unicode.MaxASCII {
			isASCII = false
			break
		}
	}

	var slug string
	if isASCII {
		slug = strings.ToLower(name)
		slug = strings.ReplaceAll(slug, " ", "-")
		slug = strings.ReplaceAll(slug, "_", "-")
		// Remove non-alphanumeric except hyphens
		reg := regexp.MustCompile(`[^a-z0-9-]`)
		slug = reg.ReplaceAllString(slug, "")
		// Collapse multiple hyphens
		for strings.Contains(slug, "--") {
			slug = strings.ReplaceAll(slug, "--", "-")
		}
		slug = strings.Trim(slug, "-")
	} else {
		slug = "proj"
	}

	suffix := strings.ReplaceAll(id.String(), "-", "")[:8]
	ns := "sb-" + slug + "-" + suffix

	if len(ns) > 63 {
		ns = ns[:63]
	}
	return ns
}

func (s *ProjectService) Create(ctx context.Context, orgID uuid.UUID, input CreateProjectInput) (*model.Project, error) {
	if !input.Environment.IsValid() {
		return nil, fmt.Errorf("invalid environment %q (must be one of prod, testing, sandbox, qa, poc, development)", input.Environment)
	}

	// The team must exist within the caller's organization.
	team, err := s.store.Teams().GetByID(ctx, input.TeamID)
	if err != nil || team.OrgID != orgID {
		return nil, fmt.Errorf("team not found")
	}

	project := &model.Project{
		OrgID:       orgID,
		TeamID:      input.TeamID,
		Name:        input.Name,
		Description: input.Description,
		Environment: input.Environment,
	}
	if err := s.store.Projects().Create(ctx, project); err != nil {
		return nil, err
	}

	// Generate and persist namespace
	project.Namespace = generateNamespace(input.Name, project.ID)
	if err := s.store.Projects().Update(ctx, project); err != nil {
		return nil, err
	}

	// Create K8s namespace
	k8s, err := defaultK8s(s.targets)
	if err != nil {
		s.logger.Error("failed to resolve default k8s target", slog.Any("error", err))
		_ = s.store.Projects().Delete(ctx, project.ID)
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}
	if err := k8s.CreateNamespace(ctx, project.Namespace); err != nil {
		s.logger.Error("failed to create K8s namespace", slog.Any("error", err), slog.String("namespace", project.Namespace))
		// Clean up the DB record — project without a namespace is unusable
		_ = s.store.Projects().Delete(ctx, project.ID)
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	// Create service account in the namespace
	saName := "sa-" + project.Namespace
	if err := k8s.EnsureServiceAccount(ctx, project.Namespace, saName); err != nil {
		s.logger.Error("failed to ensure service account", slog.Any("error", err), slog.String("namespace", project.Namespace))
		_ = s.store.Projects().Delete(ctx, project.ID)
		return nil, fmt.Errorf("failed to create service account: %w", err)
	}
	project.ServiceAccount = saName
	if err := s.store.Projects().Update(ctx, project); err != nil {
		return nil, err
	}

	s.logger.Info("project created", slog.String("name", project.Name), slog.String("id", project.ID.String()), slog.String("namespace", project.Namespace))
	return project, nil
}

func (s *ProjectService) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.store.Projects().GetByID(ctx, id)
}

func (s *ProjectService) List(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
	return s.store.Projects().ListByOrg(ctx, orgID, params)
}

// ListForUser returns the projects visible to a user: all org projects for
// owner/admin, or only the projects of the user's teams for members.
func (s *ProjectService) ListForUser(ctx context.Context, userID uuid.UUID, role string, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
	if isPrivileged(role) {
		return s.store.Projects().ListByOrg(ctx, orgID, params)
	}
	teamIDs, err := s.store.TeamMembers().ListTeamIDsByUser(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return s.store.Projects().ListByTeams(ctx, orgID, teamIDs, params)
}

func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, input UpdateProjectInput) (*model.Project, error) {
	project, err := s.store.Projects().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		project.Name = *input.Name
	}
	if input.Description != nil {
		project.Description = *input.Description
	}
	if input.Environment != nil {
		if !input.Environment.IsValid() {
			return nil, fmt.Errorf("invalid environment %q (must be one of prod, testing, sandbox, qa, poc, development)", *input.Environment)
		}
		project.Environment = *input.Environment
	}
	if input.EnvVars != nil {
		project.EnvVars = input.EnvVars
	}
	if input.ResourceQuota != nil {
		project.ResourceQuota = input.ResourceQuota
	}
	if input.NetworkPolicyEnabled != nil {
		project.NetworkPolicyEnabled = *input.NetworkPolicyEnabled
	}
	if err := s.store.Projects().Update(ctx, project); err != nil {
		return nil, err
	}

	// Sync K8s resources if project has a namespace
	if project.Namespace != "" {
		k8s, k8sErr := defaultK8s(s.targets)
		// ConfigMap for project env vars + push to running apps
		if input.EnvVars != nil {
			if k8sErr != nil {
				s.logger.Error("failed to resolve default k8s target", slog.Any("error", k8sErr))
			} else if err := k8s.EnsureConfigMap(ctx, project.Namespace, "project-env", project.EnvVars); err != nil {
				s.logger.Error("failed to ensure ConfigMap", slog.Any("error", err))
			}
			// Push merged env to all apps in this project
			apps, _, listErr := s.store.Applications().ListByProject(ctx, project.ID, store.ListParams{Page: 1, PerPage: 1000})
			if listErr != nil {
				s.logger.Error("failed to list apps for env propagation", slog.Any("error", listErr))
			} else {
				for _, app := range apps {
					merged := make(map[string]string)
					for k, v := range project.EnvVars {
						merged[k] = v
					}
					for k, v := range app.EnvVars {
						merged[k] = v
					}
					target, terr := targetForApp(s.targets, &app)
					if terr != nil {
						s.logger.Warn("failed to resolve deploy target for env propagation",
							slog.String("app", app.Name), slog.Any("error", terr))
						continue
					}
					if err := target.UpdateEnvVars(ctx, &app, merged); err != nil {
						s.logger.Warn("failed to push project env to app",
							slog.String("app", app.Name), slog.Any("error", err))
					}
				}
			}
			// Push merged env to all CronJobs in this project (merge at K8s level, don't modify DB)
			cjs, _, cjListErr := s.store.CronJobs().ListByProject(ctx, project.ID, store.ListParams{Page: 1, PerPage: 1000})
			if cjListErr != nil {
				s.logger.Error("failed to list cronjobs for env propagation", slog.Any("error", cjListErr))
			} else {
				cron, cronErr := targetCron(s.targets)
				if cronErr != nil {
					s.logger.Error("failed to resolve cron target for env propagation", slog.Any("error", cronErr))
				} else {
					for i := range cjs {
						// Build a temporary copy with merged env for K8s, don't persist
						tmp := cjs[i]
						merged := make(map[string]string)
						for k, v := range project.EnvVars {
							merged[k] = v
						}
						for k, v := range tmp.EnvVars {
							merged[k] = v
						}
						tmp.EnvVars = merged
						if err := cron.UpdateCronJob(ctx, &tmp); err != nil {
							s.logger.Warn("failed to push project env to cronjob",
								slog.String("cronjob", tmp.Name), slog.Any("error", err))
						}
					}
				}
			}
		}
		// ResourceQuota
		if input.ResourceQuota != nil {
			if k8sErr != nil {
				s.logger.Error("failed to resolve default k8s target", slog.Any("error", k8sErr))
			} else if err := k8s.EnsureResourceQuota(ctx, project.Namespace, *project.ResourceQuota); err != nil {
				s.logger.Error("failed to ensure ResourceQuota", slog.Any("error", err))
			}
		}
		// NetworkPolicy
		if input.NetworkPolicyEnabled != nil {
			if k8sErr != nil {
				s.logger.Error("failed to resolve default k8s target", slog.Any("error", k8sErr))
			} else if err := k8s.EnsureNetworkPolicy(ctx, project.Namespace, project.NetworkPolicyEnabled); err != nil {
				s.logger.Error("failed to ensure NetworkPolicy", slog.Any("error", err))
			}
		}
	}

	return project, nil
}

// UpdateEnvVars updates only the project environment variables.
func (s *ProjectService) UpdateEnvVars(ctx context.Context, id uuid.UUID, envVars map[string]string) (*model.Project, error) {
	return s.Update(ctx, id, UpdateProjectInput{EnvVars: envVars})
}

func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) error {
	project, err := s.store.Projects().GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete K8s namespace if one was assigned
	if project.Namespace != "" {
		if k8s, err := defaultK8s(s.targets); err != nil {
			s.logger.Error("failed to resolve default k8s target", slog.Any("error", err))
		} else if err := k8s.DeleteNamespace(ctx, project.Namespace); err != nil {
			s.logger.Error("failed to delete K8s namespace", slog.Any("error", err), slog.String("namespace", project.Namespace))
		}
	}

	if err := s.store.Projects().Delete(ctx, id); err != nil {
		return err
	}

	s.notifSvc.NotifyResourceDeleted(project.OrgID, model.EventProjectDeleted,
		project.Name, fmt.Sprintf("Project %q was deleted", project.Name))
	return nil
}
