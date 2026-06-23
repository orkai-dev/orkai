package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBuildService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *BuildService {
	return NewBuildService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testsupport.NewTestLogger())
}

func TestBuildSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		if opts.OnLog != nil {
			opts.OnLog("building...")
		}
		return &orchestrator.BuildResult{Image: "registry/app:abc", Logs: "done"}, nil
	}
	s := newBuildService(fs, orch)

	app := &model.Application{Name: "web", GitRepo: "https://github.com/o/r.git"}
	deploy := &model.Deployment{CommitSHA: "sha1"}
	require.NoError(t, s.Build(context.Background(), app, deploy))
	assert.Equal(t, "registry/app:abc", app.DockerImage)
	assert.Equal(t, "registry/app:abc", deploy.Image)
}

func TestBuildDefaultsBuildType(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	var gotType string
	orch.BuildFn = func(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		gotType = opts.BuildType
		return &orchestrator.BuildResult{Image: "img"}, nil
	}
	s := newBuildService(fs, orch)
	require.NoError(t, s.Build(context.Background(), &model.Application{Name: "x"}, &model.Deployment{}))
	assert.Equal(t, "dockerfile", gotType)
}

func TestBuildErrorWithLogs(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		return &orchestrator.BuildResult{Logs: "partial logs"}, errors.New("build broke")
	}
	s := newBuildService(fs, orch)
	deploy := &model.Deployment{}
	err := s.Build(context.Background(), &model.Application{Name: "x"}, deploy)
	require.Error(t, err)
	assert.Contains(t, deploy.BuildLog, "partial logs")
	assert.Contains(t, deploy.BuildLog, "Error")
}

func TestBuildErrorNoLogs(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		return nil, errors.New("build broke")
	}
	s := newBuildService(fs, orch)
	deploy := &model.Deployment{}
	err := s.Build(context.Background(), &model.Application{Name: "x"}, deploy)
	require.Error(t, err)
	assert.Equal(t, "build broke", deploy.BuildLog)
}

func TestBuildAppUpdateError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.UpdateFn = func(ctx context.Context, app *model.Application) error {
		return errors.New("update failed")
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		return &orchestrator.BuildResult{Image: "img"}, nil
	}
	s := newBuildService(fs, orch)
	err := s.Build(context.Background(), &model.Application{Name: "x"}, &model.Deployment{})
	require.ErrorContains(t, err, "update app image")
}

func TestResolveGitTokenFromProvider(t *testing.T) {
	gpID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Config: json.RawMessage(`{"token":"ghp_abc"}`)}, nil
	}
	s := newBuildService(fs, testsupport.NewFakeOrchestrator())
	token, err := s.resolveGitToken(context.Background(), &model.Application{GitProviderID: &gpID})
	require.NoError(t, err)
	assert.Equal(t, "ghp_abc", token)
}

func TestResolveGitTokenNone(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newBuildService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.resolveGitToken(context.Background(), &model.Application{Name: "x"})
	require.ErrorContains(t, err, "no git token available")
}
