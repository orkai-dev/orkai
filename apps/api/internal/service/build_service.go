package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type BuildService struct {
	store     store.Store
	targets   *orchestrator.TargetRegistry
	providers *providers.Registry
	logger    *slog.Logger
}

func NewBuildService(s store.Store, targets *orchestrator.TargetRegistry, prov *providers.Registry, logger *slog.Logger) *BuildService {
	return &BuildService{store: s, targets: targets, providers: prov, logger: logger}
}

// Build clones source code, builds a container image, and updates the app's DockerImage.
func (s *BuildService) Build(ctx context.Context, app *model.Application, deploy *model.Deployment) error {
	s.logger.Info("starting build",
		slog.String("app", app.Name),
		slog.String("repo", app.GitRepo),
		slog.String("branch", app.GitBranch),
		slog.String("buildType", string(app.BuildType)),
	)

	// Resolve git token — prefer GitHub App installation token (always fresh)
	gitToken, tokenErr := s.resolveGitToken(ctx, app)
	if tokenErr != nil {
		s.logger.Warn("failed to resolve git token, build may fail for private repos",
			slog.Any("error", tokenErr),
			slog.String("app", app.Name),
		)
	}

	opts := orchestrator.BuildOpts{
		GitRepo:      app.GitRepo,
		GitBranch:    app.GitBranch,
		CommitSHA:    deploy.CommitSHA,
		GitToken:     gitToken,
		Dockerfile:   app.Dockerfile,
		BuildContext: app.BuildContext,
		BuildArgs:    app.BuildArgs,
		BuildEnvVars: app.BuildEnvVars,
		BuildType:    string(app.BuildType),
		NoCache:      app.NoCache,
		OnLog: func(logs string) {
			// Sanitize tokens from build logs
			if gitToken != "" {
				logs = strings.ReplaceAll(logs, gitToken, "[REDACTED]")
			}
			deploy.BuildLog = logs
			_ = s.store.Deployments().Update(ctx, deploy)
		},
	}

	// Default build type to dockerfile
	if opts.BuildType == "" {
		opts.BuildType = "dockerfile"
	}

	builder, err := targetBuilder(s.targets, app)
	if err != nil {
		return err
	}
	result, err := builder.Build(ctx, app, opts)
	if err != nil {
		// Save build logs from the result (even on failure, logs may be available)
		if result != nil && result.Logs != "" {
			deploy.BuildLog = result.Logs + "\n\n--- Error ---\n" + err.Error()
		} else {
			deploy.BuildLog = err.Error()
		}
		_ = s.store.Deployments().Update(ctx, deploy)
		return err
	}

	// Update app with the built image
	app.DockerImage = result.Image
	if err := s.store.Applications().Update(ctx, app); err != nil {
		return fmt.Errorf("update app image: %w", err)
	}

	// Save build log and image on deployment
	deploy.BuildLog = result.Logs
	deploy.Image = result.Image
	_ = s.store.Deployments().Update(ctx, deploy)

	s.logger.Info("build completed",
		slog.String("app", app.Name),
		slog.String("image", result.Image),
		slog.Duration("duration", result.Duration),
	)
	return nil
}

// resolveGitToken gets a fresh token for git operations.
// Priority: GitHub App installation token > stored resource token.
func (s *BuildService) resolveGitToken(ctx context.Context, app *model.Application) (string, error) {
	// Try a fresh GitHub App installation token first (the provider mints it from
	// the configured App credentials and never expires during a build).
	if gh, err := s.providers.Git("github"); err == nil {
		if token, terr := gh.CloneToken(ctx, app.GitRepo, nil); terr == nil && token != "" {
			s.logger.Info("using GitHub App installation token", slog.String("app", app.Name))
			return token, nil
		}
	}

	// Fallback: stored token from the linked git provider resource.
	if app.GitProviderID != nil {
		resource, err := s.store.SharedResources().GetByID(ctx, *app.GitProviderID)
		if err == nil {
			var cfg struct {
				Token string `json:"token"`
			}
			if json.Unmarshal(resource.Config, &cfg) == nil && cfg.Token != "" {
				s.logger.Info("using stored git provider token", slog.String("app", app.Name))
				return cfg.Token, nil
			}
		}
	}

	return "", fmt.Errorf("no git token available for %s", app.Name)
}
