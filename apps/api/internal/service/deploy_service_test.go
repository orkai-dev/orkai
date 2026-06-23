package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDeployService builds a DeployService without background schedulers.
func newDeployService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *DeployService {
	bs := NewBuildService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testsupport.NewTestLogger())
	ra := NewRegistryAuth(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testsupport.NewTestLogger())
	if fs.ProjectsStore.GetByIDFn == nil {
		fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
			return &model.Project{}, nil
		}
	}
	return &DeployService{
		store:        fs,
		targets:      testsupport.NewFakeTargetRegistry(orch),
		logger:       testsupport.NewTestLogger(),
		buildSvc:     bs,
		registryAuth: ra,
		queue:        testsupport.NewFakeEnqueuer(),
	}
}

func TestMinLen(t *testing.T) {
	assert.Equal(t, 3, minLen(3, 5))
	assert.Equal(t, 5, minLen(7, 5))
}

func TestExecuteDeployImageSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "web", SourceType: model.SourceImage, DockerImage: "nginx", Status: model.AppStatusBuilding}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID}
	s.executeDeploy(context.Background(), app, deploy, false)
	assert.Equal(t, model.DeploySuccess, deploy.Status)
}

func TestExecuteDeployImageWithHPA(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	hpaCalled := false
	orch.ConfigureHPAFn = func(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error {
		hpaCalled = true
		return nil
	}
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "web", SourceType: model.SourceImage, DockerImage: "nginx",
		Autoscaling: &model.AutoscalingConfig{Enabled: true, MinReplicas: 1, MaxReplicas: 3}}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID}
	s.executeDeploy(context.Background(), app, deploy, false)
	assert.True(t, hpaCalled)
}

func TestExecuteDeployHPAFailureStillSucceeds(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.ConfigureHPAFn = func(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error {
		return errors.New("hpa failed")
	}
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, SourceType: model.SourceImage,
		Autoscaling: &model.AutoscalingConfig{Enabled: true, MinReplicas: 1, MaxReplicas: 3}}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID}
	s.executeDeploy(context.Background(), app, deploy, false)
	assert.Equal(t, model.DeploySuccess, deploy.Status)
}

func TestExecuteDeployOrchError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.DeployFn = func(ctx context.Context, app *model.Application, opts orchestrator.DeployOpts) error {
		return errors.New("deploy failed")
	}
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, SourceType: model.SourceImage}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID}
	s.executeDeploy(context.Background(), app, deploy, false)
	assert.Equal(t, model.DeployFailed, deploy.Status)
}

func TestExecuteDeployRegistryError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	regID := uuid.New()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return nil, errors.New("registry gone")
	}
	orch := testsupport.NewFakeOrchestrator()
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, SourceType: model.SourceImage, RegistryID: &regID}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID}
	s.executeDeploy(context.Background(), app, deploy, false)
	assert.Equal(t, model.DeployFailed, deploy.Status)
}

func TestExecuteDeployGitBuildSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "web", SourceType: model.SourceGit, GitRepo: "https://github.com/o/r"}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return app, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, a *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		return &orchestrator.BuildResult{Image: "built:abc"}, nil
	}
	s := newDeployService(fs, orch)
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID}
	s.executeDeploy(context.Background(), app, deploy, true)
	assert.Equal(t, model.DeploySuccess, deploy.Status)
}

func TestExecuteDeployGitBuildFailure(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, a *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		return nil, errors.New("build failed")
	}
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, SourceType: model.SourceGit, GitRepo: "https://github.com/o/r"}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID}
	s.executeDeploy(context.Background(), app, deploy, true)
	assert.Equal(t, model.DeployFailed, deploy.Status)
}

// TestExecuteDeployBuildErrorWithCancelRequested covers the cancel race: Cancel()
// deletes the build job (producing a non-context build error) before the cancel
// poll fires cancel(). The DB cancel flag must still finalize the deploy as
// cancelled rather than failed.
func TestExecuteDeployBuildErrorWithCancelRequested(t *testing.T) {
	deployID := uuid.New()
	appID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{BaseModel: model.BaseModel{ID: deployID}, AppID: appID, CancelRequested: true}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, a *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		return nil, errors.New("build job deleted")
	}
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: appID}, SourceType: model.SourceGit, GitRepo: "https://github.com/o/r"}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: deployID}, AppID: appID}
	s.executeDeploy(context.Background(), app, deploy, true)
	assert.Equal(t, model.DeployCancelled, deploy.Status)
}

// TestExecuteDeployOrchErrorWithCancelRequested covers the same race during the
// deploy phase: an orchestrator error while cancel was requested must finalize
// as cancelled, not failed.
func TestExecuteDeployOrchErrorWithCancelRequested(t *testing.T) {
	deployID := uuid.New()
	appID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{BaseModel: model.BaseModel{ID: deployID}, AppID: appID, CancelRequested: true}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeployFn = func(ctx context.Context, app *model.Application, opts orchestrator.DeployOpts) error {
		return errors.New("deploy failed")
	}
	s := newDeployService(fs, orch)
	app := &model.Application{BaseModel: model.BaseModel{ID: appID}, SourceType: model.SourceImage}
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: deployID}, AppID: appID}
	s.executeDeploy(context.Background(), app, deploy, false)
	assert.Equal(t, model.DeployCancelled, deploy.Status)
}

func TestExecuteDeploySkipBuildSameCommit(t *testing.T) {
	fs := testsupport.NewFakeStore()
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, SourceType: model.SourceGit, GitRepo: "r"}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return app, nil
	}
	fs.DeploymentsStore.GetLatestByAppFn = func(ctx context.Context, appID uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{Status: model.DeploySuccess, Image: "old:img", CommitSHA: "abc1234"}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.BuildFn = func(ctx context.Context, a *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
		t.Fatal("build should be skipped")
		return nil, nil
	}
	s := newDeployService(fs, orch)
	deploy := &model.Deployment{BaseModel: model.BaseModel{ID: uuid.New()}, AppID: app.ID, CommitSHA: "abc1234"}
	s.executeDeploy(context.Background(), app, deploy, false)
	assert.Equal(t, model.DeploySuccess, deploy.Status)
	assert.Equal(t, "old:img", deploy.Image)
}

func TestTriggerAppNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, errors.New("missing")
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Trigger(context.Background(), TriggerDeployInput{AppID: uuid.New()})
	require.Error(t, err)
}

func TestTriggerAppBusy(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Status: model.AppStatusBuilding}, nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Trigger(context.Background(), TriggerDeployInput{AppID: uuid.New()})
	require.ErrorContains(t, err, "currently")
}

func TestTriggerCreateError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, SourceType: model.SourceImage}, nil
	}
	fs.DeploymentsStore.CreateFn = func(ctx context.Context, d *model.Deployment) error {
		return errors.New("insert failed")
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Trigger(context.Background(), TriggerDeployInput{AppID: uuid.New()})
	require.Error(t, err)
}

func TestTriggerSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, SourceType: model.SourceImage, DockerImage: "nginx", Status: model.AppStatusIdle}, nil
	}
	deployID := uuid.New()
	fs.DeploymentsStore.CreateFn = func(ctx context.Context, d *model.Deployment) error {
		d.ID = deployID
		return nil
	}
	eq := testsupport.NewFakeEnqueuer()
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	s.queue = eq
	deploy, err := s.Trigger(context.Background(), TriggerDeployInput{AppID: uuid.New(), TriggerType: "manual", ForceBuild: true})
	require.NoError(t, err)
	assert.Equal(t, model.DeployQueued, deploy.Status)
	job, ok := eq.LastJob()
	require.True(t, ok)
	assert.Equal(t, jobs.JobDeploy, job.Type)
	require.NotNil(t, job.DeployID)
	assert.Equal(t, deployID, *job.DeployID)
	assert.True(t, job.ForceBuild)
}

func TestCancelGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return nil, errors.New("missing")
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	require.Error(t, s.Cancel(context.Background(), uuid.New()))
}

func TestCancelNotInProgress(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{Status: model.DeploySuccess}, nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	require.ErrorContains(t, s.Cancel(context.Background(), uuid.New()), "not in progress")
}

func TestCancelSuccess(t *testing.T) {
	deployID := uuid.New()
	appID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{BaseModel: model.BaseModel{ID: deployID}, Status: model.DeployBuilding, AppID: appID}, nil
	}
	updated := false
	fs.DeploymentsStore.UpdateFn = func(ctx context.Context, d *model.Deployment) error {
		updated = true
		assert.True(t, d.CancelRequested)
		return nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web"}, nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Cancel(context.Background(), deployID))
	assert.True(t, updated)
}
func TestCancelQueuedFinalizesImmediately(t *testing.T) {
	deployID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{BaseModel: model.BaseModel{ID: deployID}, Status: model.DeployQueued, AppID: uuid.New()}, nil
	}
	var saved *model.Deployment
	fs.DeploymentsStore.UpdateFn = func(ctx context.Context, d *model.Deployment) error {
		saved = d
		return nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web"}, nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Cancel(context.Background(), deployID))
	require.NotNil(t, saved)
	assert.True(t, saved.CancelRequested)
	assert.Equal(t, model.DeployCancelled, saved.Status)
}

func TestRunDeployJobSkipsTerminal(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{Status: model.DeploySuccess}, nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.RunDeployJob(context.Background(), uuid.New(), false))
}

func TestRunDeployJobHonorsCancelFlag(t *testing.T) {
	deployID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{
			BaseModel:       model.BaseModel{ID: deployID},
			Status:          model.DeployQueued,
			CancelRequested: true,
			AppID:           uuid.New(),
		}, nil
	}
	var saved *model.Deployment
	fs.DeploymentsStore.UpdateFn = func(ctx context.Context, d *model.Deployment) error {
		saved = d
		return nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.RunDeployJob(context.Background(), deployID, false))
	require.NotNil(t, saved)
	assert.Equal(t, model.DeployCancelled, saved.Status)
}

func TestDeployGetByIDAndList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	did := uuid.New()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{BaseModel: model.BaseModel{ID: did}}, nil
	}
	fs.DeploymentsStore.ListByAppFn = func(ctx context.Context, appID uuid.UUID, p store.ListParams) ([]model.Deployment, int, error) {
		return []model.Deployment{{}}, 1, nil
	}
	fs.DeploymentsStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.DeploymentListFilter) ([]model.Deployment, int, error) {
		return []model.Deployment{{}}, 1, nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())

	got, err := s.GetByID(context.Background(), did)
	require.NoError(t, err)
	assert.Equal(t, did, got.ID)

	_, total, err := s.List(context.Background(), uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	_, total, err = s.ListAll(context.Background(), store.DefaultListParams(), store.DeploymentListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestRollbackSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{AppID: uuid.New(), Image: "v1:img", CommitSHA: "sha"}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web"}, nil
	}
	fs.DeploymentsStore.CreateFn = func(ctx context.Context, d *model.Deployment) error {
		d.ID = uuid.New()
		return nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	deploy, err := s.Rollback(context.Background(), uuid.New(), nil)
	require.NoError(t, err)
	assert.Equal(t, model.DeploySuccess, deploy.Status)
	assert.Equal(t, "v1:img", deploy.Image)
}

func TestRollbackPrevGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return nil, errors.New("missing")
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Rollback(context.Background(), uuid.New(), nil)
	require.Error(t, err)
}

func TestRollbackOrchError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{AppID: uuid.New()}, nil
	}
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.RollbackFn = func(ctx context.Context, app *model.Application, revision int64) error {
		return errors.New("rollback failed")
	}
	s := newDeployService(fs, orch)
	_, err := s.Rollback(context.Background(), uuid.New(), nil)
	require.Error(t, err)
}

func TestRecoverStaleDeployments(t *testing.T) {
	fs := testsupport.NewFakeStore()
	old := time.Now().Add(-1 * time.Hour)
	recent := time.Now()
	oldCreatedRecentStarted := time.Now().Add(-2 * time.Hour)
	fs.DeploymentsStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.DeploymentListFilter) ([]model.Deployment, int, error) {
		if f.Status == "building" {
			return []model.Deployment{
				{BaseModel: model.BaseModel{ID: uuid.New(), CreatedAt: old}, StartedAt: &recent, AppName: "queued-long-running", AppID: uuid.New()},
				{BaseModel: model.BaseModel{ID: uuid.New(), CreatedAt: recent}, StartedAt: &old, AppName: "stale", AppID: uuid.New()},
				{BaseModel: model.BaseModel{ID: uuid.New(), CreatedAt: recent}, StartedAt: &recent, AppName: "fresh", AppID: uuid.New()},
				{BaseModel: model.BaseModel{ID: uuid.New(), CreatedAt: oldCreatedRecentStarted}, AppName: "legacy-no-started-at", AppID: uuid.New()},
			}, 4, nil
		}
		return nil, 0, nil
	}
	updated := 0
	fs.DeploymentsStore.UpdateFn = func(ctx context.Context, d *model.Deployment) error {
		updated++
		return nil
	}
	s := newDeployService(fs, testsupport.NewFakeOrchestrator())
	s.recoverStaleDeployments(context.Background())
	assert.Equal(t, 2, updated, "only deployments stale by StartedAt (or CreatedAt fallback) should be updated")
}
