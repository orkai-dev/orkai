package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCronJobService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *CronJobService {
	return NewCronJobService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewTestLogger(), nil)
}

func TestCronJobCreateProjectNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, errors.New("missing")
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateCronJobInput{})
	require.Error(t, err)
}

func TestCronJobCreateGitSourceRejected(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateCronJobInput{SourceType: string(model.SourceGit)})
	require.ErrorContains(t, err, "do not support git")
}

func TestCronJobCreateSuccessDefaults(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	cj, err := s.Create(context.Background(), CreateCronJobInput{Name: "job", CronExpression: "* * * * *", Command: "echo hi"})
	require.NoError(t, err)
	assert.Equal(t, "UTC", cj.Timezone)
	assert.Equal(t, "busybox:latest", cj.Image)
	assert.Equal(t, "Forbid", cj.ConcurrencyPolicy)
	assert.Equal(t, 3, cj.BackoffLimit)
	assert.NotNil(t, cj.EnvVars)
}

func TestCronJobCreateExplicitZeroBackoffLimit(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	zero := 0
	cj, err := s.Create(context.Background(), CreateCronJobInput{
		Name: "job", CronExpression: "* * * * *", Command: "echo hi", BackoffLimit: &zero,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, cj.BackoffLimit)
}

func TestCronJobCreateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	fs.CronJobsStore.CreateFn = func(ctx context.Context, cj *model.CronJob) error {
		return errors.New("insert failed")
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateCronJobInput{Name: "job", CronExpression: "* * * * *", Command: "echo"})
	require.Error(t, err)
}

func TestCronJobCreateK8sFailRollback(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns", EnvVars: map[string]string{"BASE": "1"}}, nil
	}
	deleted := false
	fs.CronJobsStore.DeleteFn = func(ctx context.Context, id uuid.UUID) error {
		deleted = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.CreateCronJobFn = func(ctx context.Context, cj *model.CronJob) error {
		return errors.New("k8s failed")
	}
	s := newCronJobService(fs, orch)
	_, err := s.Create(context.Background(), CreateCronJobInput{Name: "job", CronExpression: "* * * * *", Command: "echo"})
	require.ErrorContains(t, err, "failed to create K8s CronJob")
	assert.True(t, deleted)
}

func TestCronJobUpdate(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{Name: "job"}, nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{EnvVars: map[string]string{"BASE": "1"}}, nil
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	expr, tz, cmd, img := "0 0 * * *", "America/New_York", "run.sh", "alpine"
	enabled := true
	backoff, deadline := 5, 600
	desc := "nightly"
	policy, restart := "Allow", "Never"
	cj, err := s.Update(context.Background(), uuid.New(), UpdateCronJobInput{
		CronExpression: &expr, Timezone: &tz, Command: &cmd, Image: &img,
		EnvVars: map[string]string{"K": "V"}, CPULimit: &cmd, MemLimit: &cmd,
		Enabled: &enabled, ConcurrencyPolicy: &policy, RestartPolicy: &restart,
		BackoffLimit: &backoff, ActiveDeadlineSeconds: &deadline, Description: &desc,
	})
	require.NoError(t, err)
	assert.Equal(t, "0 0 * * *", cj.CronExpression)
}

func TestCronJobUpdateGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return nil, errors.New("missing")
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), UpdateCronJobInput{})
	require.Error(t, err)
}

func TestCronJobUpdateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{}, nil
	}
	fs.CronJobsStore.UpdateFn = func(ctx context.Context, cj *model.CronJob) error {
		return errors.New("update failed")
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), UpdateCronJobInput{})
	require.Error(t, err)
}

func TestCronJobUpdateK8sError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.UpdateCronJobFn = func(ctx context.Context, cj *model.CronJob) error {
		return errors.New("k8s failed")
	}
	s := newCronJobService(fs, orch)
	_, err := s.Update(context.Background(), uuid.New(), UpdateCronJobInput{})
	require.ErrorContains(t, err, "failed to update K8s CronJob")
}

func TestCronJobGetListRuns(t *testing.T) {
	fs := testsupport.NewFakeStore()
	cid := uuid.New()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{BaseModel: model.BaseModel{ID: cid}}, nil
	}
	fs.CronJobsStore.ListByProjectFn = func(ctx context.Context, pid uuid.UUID, p store.ListParams) ([]model.CronJob, int, error) {
		return []model.CronJob{{}}, 1, nil
	}
	fs.CronJobRunsStore.ListByCronJobFn = func(ctx context.Context, cjid uuid.UUID, p store.ListParams) ([]model.CronJobRun, int, error) {
		return []model.CronJobRun{{}}, 1, nil
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())

	got, err := s.GetByID(context.Background(), cid)
	require.NoError(t, err)
	assert.Equal(t, cid, got.ID)

	_, total, err := s.List(context.Background(), uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	_, total, err = s.ListRuns(context.Background(), cid, store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestCronJobDelete(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{}, nil
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Delete(context.Background(), uuid.New()))
}

func TestCronJobDeleteGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return nil, errors.New("missing")
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	require.Error(t, s.Delete(context.Background(), uuid.New()))
}

func TestCronJobDeleteK8sError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeleteCronJobFn = func(ctx context.Context, cj *model.CronJob) error {
		return errors.New("k8s failed")
	}
	s := newCronJobService(fs, orch)
	require.ErrorContains(t, s.Delete(context.Background(), uuid.New()), "delete manually")
}

func TestCronJobTriggerGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return nil, errors.New("missing")
	}
	s := newCronJobService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Trigger(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestCronJobTriggerOrchError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.TriggerCronJobFn = func(ctx context.Context, cj *model.CronJob) (string, error) {
		return "", errors.New("trigger failed")
	}
	s := newCronJobService(fs, orch)
	_, err := s.Trigger(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestCronJobTriggerSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{Name: "job"}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.TriggerCronJobFn = func(ctx context.Context, cj *model.CronJob) (string, error) {
		return "job-123", nil
	}
	// Keep watchJobCompletion goroutine from completing during the test.
	orch.GetJobStatusFn = func(ctx context.Context, cj *model.CronJob, jobName string) (string, error) {
		return "", errors.New("pending")
	}
	s := newCronJobService(fs, orch)
	run, err := s.Trigger(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, model.CronJobRunRunning, run.Status)
	assert.Equal(t, "manual", run.TriggerType)
}

func TestCronJobTriggerRunCreateError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{}, nil
	}
	fs.CronJobRunsStore.CreateFn = func(ctx context.Context, run *model.CronJobRun) error {
		return errors.New("insert failed")
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.TriggerCronJobFn = func(ctx context.Context, cj *model.CronJob) (string, error) {
		return "job-123", nil
	}
	s := newCronJobService(fs, orch)
	_, err := s.Trigger(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestWithMergedEnvAndProjectEnvOrNil(t *testing.T) {
	s := newCronJobService(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	cj := &model.CronJob{EnvVars: map[string]string{"A": "cj"}}

	// empty project env returns same pointer
	assert.Same(t, cj, s.withMergedEnv(cj, nil))

	merged := s.withMergedEnv(cj, map[string]string{"A": "proj", "B": "proj"})
	assert.Equal(t, "cj", merged.EnvVars["A"]) // cj overrides
	assert.Equal(t, "proj", merged.EnvVars["B"])

	assert.Nil(t, projectEnvOrNil(nil))
	assert.NotNil(t, projectEnvOrNil(&model.Project{EnvVars: map[string]string{}}))
}
