package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAppService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *AppService {
	ra := NewRegistryAuth(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testsupport.NewTestLogger())
	return NewAppService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewTestLogger(), nil, ra, nil)
}

func TestGuessContainerPort(t *testing.T) {
	cases := map[string]int{
		"nginx:latest": 80, "node:20": 3000, "django-app": 8000,
		"spring-boot": 8080, "postgres:16": 5432, "mysql:8": 3306,
		"redis:7": 6379, "mongo:7": 8080, "rails-app": 3000,
		"some-random-image": 80, "valkey": 6379, "tomcat": 8080,
		"flask-api": 8000,
	}
	for img, want := range cases {
		assert.Equal(t, want, guessContainerPort(img), "image %s", img)
	}
}

func TestSanitizeK8sName(t *testing.T) {
	assert.Equal(t, "my-app", sanitizeK8sName("My_App"))
	assert.Equal(t, "a-b", sanitizeK8sName("a b"))
	long := sanitizeK8sName(string(make([]byte, 100)))
	assert.LessOrEqual(t, len(long), 63)
}

func TestRandomHex(t *testing.T) {
	assert.Len(t, randomHex(8), 16)
}

func validProject(orgID uuid.UUID) *model.Project {
	return &model.Project{BaseModel: model.BaseModel{ID: uuid.New()}, OrgID: orgID, Namespace: "sb-test-1234"}
}

func TestAppCreateProjectNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, errors.New("missing")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "nginx"})
	require.ErrorContains(t, err, "project not found")
}

func TestAppCreateNoNamespace(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "nginx"})
	require.ErrorContains(t, err, "no namespace")
}

func TestAppCreateSuccess(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return validProject(orgID), nil
	}
	fs.ApplicationsStore.CreateFn = func(ctx context.Context, app *model.Application) error {
		app.ID = uuid.New()
		return nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	app, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "nginx"})
	require.NoError(t, err)
	assert.Equal(t, model.AppStatusIdle, app.Status)
	assert.Equal(t, "main", app.GitBranch)
	assert.Equal(t, int32(1), app.Replicas)
	assert.Len(t, app.Ports, 1)
	assert.Equal(t, 80, app.Ports[0].ContainerPort)
}

func TestAppCreateNameConflictApp(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return validProject(orgID), nil
	}
	fs.ApplicationsStore.ExistsByK8sNameFn = func(ctx context.Context, pid uuid.UUID, k8sName string) (bool, error) {
		return true, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "nginx"})
	require.ErrorContains(t, err, "already exists")
}

func TestAppCreateNameConflictDatabase(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return validProject(orgID), nil
	}
	fs.ManagedDatabasesStore.ExistsByK8sNameFn = func(ctx context.Context, pid uuid.UUID, k8sName string) (bool, error) {
		return true, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "nginx"})
	require.ErrorContains(t, err, "database with K8s name")
}

func TestAppCreateGitProviderChecks(t *testing.T) {
	orgID := uuid.New()
	gpID := uuid.New()

	// not found
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return validProject(orgID), nil
	}
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return nil, errors.New("missing")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceGit, GitRepo: "r", GitProviderID: &gpID})
	require.ErrorContains(t, err, "git provider not found")

	// wrong org
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: uuid.New()}, nil
	}
	_, err = s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceGit, GitRepo: "r", GitProviderID: &gpID})
	require.ErrorContains(t, err, "does not belong")
}

func TestAppCreateRegistryChecks(t *testing.T) {
	orgID := uuid.New()
	regID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return validProject(orgID), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return nil, errors.New("missing")
	}
	_, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "x", RegistryID: &regID})
	require.ErrorContains(t, err, "registry not found")

	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: uuid.New(), Type: model.ResourceRegistry}, nil
	}
	_, err = s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "x", RegistryID: &regID})
	require.ErrorContains(t, err, "does not belong")

	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceSSHKey}, nil
	}
	_, err = s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "x", RegistryID: &regID})
	require.ErrorContains(t, err, "not a registry")
}

func TestAppCreateStoreError(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return validProject(orgID), nil
	}
	fs.ApplicationsStore.CreateFn = func(ctx context.Context, app *model.Application) error {
		return errors.New("insert failed")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), CreateAppInput{ProjectID: uuid.New(), Name: "web", SourceType: model.SourceImage, DockerImage: "nginx"})
	require.Error(t, err)
}

func appWithStatus(status model.AppStatus) *model.Application {
	return &model.Application{
		BaseModel:  model.BaseModel{ID: uuid.New()},
		Name:       "web",
		SourceType: model.SourceImage,
		Status:     status,
	}
}

func TestAppUpdateGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, errors.New("missing")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{})
	require.Error(t, err)
}

func TestAppUpdateValidationErrors(t *testing.T) {
	mkApp := func() *model.Application {
		return &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "web", SourceType: model.SourceGit, GitRepo: "r"}
	}
	empty := "  "
	badType := "weird"
	tests := []struct {
		name  string
		input UpdateAppInput
		want  string
	}{
		{"empty git repo", UpdateAppInput{GitRepo: &empty}, "git repo cannot be empty"},
		{"empty git branch", UpdateAppInput{GitBranch: &empty}, "git branch cannot be empty"},
		{"bad deploy strategy", UpdateAppInput{DeployStrategy: &badType}, "deploy strategy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := testsupport.NewFakeStore()
			fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
				return mkApp(), nil
			}
			s := newAppService(fs, testsupport.NewFakeOrchestrator())
			_, err := s.Update(context.Background(), uuid.New(), tc.input)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestAppUpdateHealthCheckValidation(t *testing.T) {
	tests := []struct {
		name string
		hc   *model.HealthCheck
		want string
	}{
		{"http no path", &model.HealthCheck{Type: "http", Port: 80}, "path is required"},
		{"http bad port", &model.HealthCheck{Type: "http", Path: "/", Port: 0}, "port must be between"},
		{"tcp bad port", &model.HealthCheck{Type: "tcp", Port: 99999}, "port must be between"},
		{"exec no command", &model.HealthCheck{Type: "exec"}, "command is required"},
		{"invalid type", &model.HealthCheck{Type: "weird"}, "invalid health check type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := testsupport.NewFakeStore()
			fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
				return appWithStatus(model.AppStatusIdle), nil
			}
			s := newAppService(fs, testsupport.NewFakeOrchestrator())
			_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{HealthCheck: tc.hc})
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestAppUpdatePortAndResourceValidation(t *testing.T) {
	newSvc := func() *AppService {
		fs := testsupport.NewFakeStore()
		fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
			return appWithStatus(model.AppStatusIdle), nil
		}
		return newAppService(fs, testsupport.NewFakeOrchestrator())
	}

	_, err := newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{Ports: []model.PortMapping{{ContainerPort: 0}}})
	require.ErrorContains(t, err, "container port")

	_, err = newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{Ports: []model.PortMapping{{ContainerPort: 80, Protocol: "sctp"}}})
	require.ErrorContains(t, err, "protocol must be")

	badCPU := "abc"
	_, err = newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{CPULimit: &badCPU})
	require.ErrorContains(t, err, "invalid cpu_limit")

	neg := -5
	_, err = newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{TerminationGracePeriod: &neg})
	require.ErrorContains(t, err, "grace period cannot be negative")
}

func TestAppUpdateVolumeValidation(t *testing.T) {
	newSvc := func() *AppService {
		fs := testsupport.NewFakeStore()
		fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
			return appWithStatus(model.AppStatusIdle), nil
		}
		return newAppService(fs, testsupport.NewFakeOrchestrator())
	}
	tests := []struct {
		name string
		vols []model.VolumeMount
		want string
	}{
		{"empty name", []model.VolumeMount{{MountPath: "/data"}}, "volume name cannot be empty"},
		{"dup name", []model.VolumeMount{{Name: "v", MountPath: "/a"}, {Name: "v", MountPath: "/b"}}, "duplicate volume name"},
		{"empty mount", []model.VolumeMount{{Name: "v"}}, "mount path cannot be empty"},
		{"relative mount", []model.VolumeMount{{Name: "v", MountPath: "data"}}, "must be absolute"},
		{"dup mount", []model.VolumeMount{{Name: "v1", MountPath: "/a"}, {Name: "v2", MountPath: "/a"}}, "duplicate volume mount path"},
		{"bad size", []model.VolumeMount{{Name: "v", MountPath: "/a", Size: "xx"}}, "invalid volume size"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{Volumes: tc.vols})
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestAppUpdateVolumeStorageClassValidation(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	target, err := fs.DeployTargets().GetDefault(context.Background())
	require.NoError(t, err)
	target.Config.AllowedStorageClasses = []string{"gp3", "local-path"}
	require.NoError(t, fs.DeployTargets().Update(context.Background(), target))

	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	_, err = s.Update(context.Background(), uuid.New(), UpdateAppInput{
		Volumes: []model.VolumeMount{{Name: "data", MountPath: "/data", Size: "5Gi", StorageClass: "io2"}},
	})
	require.ErrorContains(t, err, "storage class")

	_, err = s.Update(context.Background(), uuid.New(), UpdateAppInput{
		Volumes: []model.VolumeMount{{Name: "data", MountPath: "/data", Size: "5Gi", StorageClass: "gp3"}},
	})
	require.NoError(t, err)
}

func TestAppUpdateVolumeStorageClassEmptyAllowlist(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{
		Volumes: []model.VolumeMount{{Name: "data", MountPath: "/data", Size: "5Gi", StorageClass: "custom-class"}},
	})
	require.NoError(t, err)
}

func TestAppUpdateDeployTargetLookupFailure(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	fs.DeployTargetsStore.GetDefaultFn = func(ctx context.Context) (*model.DeployTarget, error) {
		return nil, errors.New("db unavailable")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{
		Volumes: []model.VolumeMount{{Name: "data", MountPath: "/data", Size: "5Gi", StorageClass: "io2"}},
	})
	require.ErrorContains(t, err, "deploy target")
}

func TestAppUpdateDeployTargetLookupFailureNoStorageClassOverride(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	fs.DeployTargetsStore.GetDefaultFn = func(ctx context.Context) (*model.DeployTarget, error) {
		return nil, errors.New("db unavailable")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	cpuLim := "1"
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{CPULimit: &cpuLim})
	require.NoError(t, err)

	_, err = s.Update(context.Background(), uuid.New(), UpdateAppInput{
		Volumes: []model.VolumeMount{{Name: "data", MountPath: "/data", Size: "5Gi"}},
	})
	require.NoError(t, err)
}

func TestAppGetTargetCapabilitiesDeployTargetLookupFailure(t *testing.T) {
	fs := testsupport.NewFakeStore()
	appID := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	fs.DeployTargetsStore.GetDefaultFn = func(ctx context.Context) (*model.DeployTarget, error) {
		return nil, errors.New("db unavailable")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	_, err := s.GetTargetCapabilities(context.Background(), appID)
	require.ErrorContains(t, err, "deploy target")
}

func TestAppUpdateRequestExceedsLimit(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	cpuReq, cpuLim := "2", "1"
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{CPURequest: &cpuReq, CPULimit: &cpuLim})
	require.ErrorContains(t, err, "cpu_request")

	memReq, memLim := "2Gi", "1Gi"
	_, err = s.Update(context.Background(), uuid.New(), UpdateAppInput{MemRequest: &memReq, MemLimit: &memLim})
	require.ErrorContains(t, err, "mem_request")
}

func TestAppUpdateAutoscalingValidation(t *testing.T) {
	newSvc := func() *AppService {
		fs := testsupport.NewFakeStore()
		fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
			return appWithStatus(model.AppStatusIdle), nil
		}
		return newAppService(fs, testsupport.NewFakeOrchestrator())
	}
	_, err := newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{Autoscaling: &model.AutoscalingConfig{Enabled: true, MinReplicas: 0}})
	require.ErrorContains(t, err, "min replicas")

	_, err = newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{Autoscaling: &model.AutoscalingConfig{Enabled: true, MinReplicas: 3, MaxReplicas: 1}})
	require.ErrorContains(t, err, "max replicas")

	_, err = newSvc().Update(context.Background(), uuid.New(), UpdateAppInput{Autoscaling: &model.AutoscalingConfig{Enabled: true, MinReplicas: 1, MaxReplicas: 3}})
	require.ErrorContains(t, err, "metric target")
}

func TestAppUpdateSuccessNoRuntimeChange(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	crit := true
	app, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{IsCritical: &crit})
	require.NoError(t, err)
	assert.True(t, app.IsCritical)
}

func TestAppUpdateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	fs.ApplicationsStore.UpdateFn = func(ctx context.Context, app *model.Application) error {
		return errors.New("update failed")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{})
	require.Error(t, err)
}

func TestAppUpdateAutoscalingEnableAndDisable(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	orch := testsupport.NewFakeOrchestrator()
	s := newAppService(fs, orch)
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{Autoscaling: &model.AutoscalingConfig{Enabled: true, MinReplicas: 1, MaxReplicas: 3, CPUTarget: 70}})
	require.NoError(t, err)

	_, err = s.Update(context.Background(), uuid.New(), UpdateAppInput{Autoscaling: &model.AutoscalingConfig{Enabled: false}})
	require.NoError(t, err)
}

func TestAppUpdateHPAErrorRollsBack(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.ConfigureHPAFn = func(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error {
		return errors.New("hpa failed")
	}
	s := newAppService(fs, orch)
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{Autoscaling: &model.AutoscalingConfig{Enabled: true, MinReplicas: 1, MaxReplicas: 3, CPUTarget: 70}})
	require.ErrorContains(t, err, "failed to apply HPA")
}

func TestAppUpdateRuntimeChangeRedeploys(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{EnvVars: map[string]string{"G": "1"}}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	deployed := false
	orch.DeployFn = func(ctx context.Context, app *model.Application, opts orchestrator.DeployOpts) error {
		deployed = true
		return nil
	}
	s := newAppService(fs, orch)
	cpu := "250m"
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{CPULimit: &cpu})
	require.NoError(t, err)
	assert.True(t, deployed)
}

func TestAppUpdateRuntimeChangeDeployErrorRollsBack(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeployFn = func(ctx context.Context, app *model.Application, opts orchestrator.DeployOpts) error {
		return errors.New("deploy failed")
	}
	s := newAppService(fs, orch)
	cpu := "250m"
	_, err := s.Update(context.Background(), uuid.New(), UpdateAppInput{CPULimit: &cpu})
	require.ErrorContains(t, err, "failed to apply deployment changes")
}

func TestAppGetByIDAndListAndListAll(t *testing.T) {
	fs := testsupport.NewFakeStore()
	aid := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: aid}}, nil
	}
	fs.ApplicationsStore.ListByProjectFn = func(ctx context.Context, pid uuid.UUID, p store.ListParams) ([]model.Application, int, error) {
		return []model.Application{*appWithStatus(model.AppStatusRunning)}, 1, nil
	}
	fs.ApplicationsStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.AppListFilter) ([]model.Application, int, error) {
		return []model.Application{*appWithStatus(model.AppStatusIdle)}, 1, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	got, err := s.GetByID(context.Background(), aid)
	require.NoError(t, err)
	assert.Equal(t, aid, got.ID)

	_, total, err := s.List(context.Background(), uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	_, total, err = s.ListAll(context.Background(), store.DefaultListParams(), store.AppListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestAppDelete(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	fs.DomainsStore.ListByAppFn = func(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
		return []model.Domain{{Host: "x.example.com"}}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeleteFn = func(ctx context.Context, app *model.Application) error { return errors.New("k8s err") }
	s := newAppService(fs, orch)
	require.NoError(t, s.Delete(context.Background(), uuid.New()))
}

func TestAppDeleteGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, errors.New("missing")
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	require.Error(t, s.Delete(context.Background(), uuid.New()))
}

func TestAppScale(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	app, err := s.Scale(context.Background(), uuid.New(), 3)
	require.NoError(t, err)
	assert.Equal(t, int32(3), app.Replicas)
}

func TestAppScaleHPAActive(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		a := appWithStatus(model.AppStatusRunning)
		a.Autoscaling = &model.AutoscalingConfig{Enabled: true}
		return a, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Scale(context.Background(), uuid.New(), 3)
	require.ErrorContains(t, err, "autoscaling")
}

func TestAppScaleOutOfRange(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Scale(context.Background(), uuid.New(), 200)
	require.ErrorContains(t, err, "between 0 and 100")
}

func TestAppScaleOrchError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.ScaleFn = func(ctx context.Context, app *model.Application, replicas int32) error {
		return errors.New("scale failed")
	}
	s := newAppService(fs, orch)
	_, err := s.Scale(context.Background(), uuid.New(), 3)
	require.Error(t, err)
}

func TestAppUpdateEnvVars(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	pushed := false
	orch.UpdateEnvVarsFn = func(ctx context.Context, app *model.Application, envVars map[string]string) error {
		pushed = true
		return nil
	}
	s := newAppService(fs, orch)
	_, err := s.UpdateEnvVars(context.Background(), uuid.New(), map[string]string{"K": "V"})
	require.NoError(t, err)
	assert.True(t, pushed)
}

func TestAppUpdateEnvVarsIdleNoPush(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.UpdateEnvVarsFn = func(ctx context.Context, app *model.Application, envVars map[string]string) error {
		t.Fatal("should not push when idle")
		return nil
	}
	s := newAppService(fs, orch)
	_, err := s.UpdateEnvVars(context.Background(), uuid.New(), map[string]string{"K": "V"})
	require.NoError(t, err)
}

func TestAppUpdateEnvVarsPushError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.UpdateEnvVarsFn = func(ctx context.Context, app *model.Application, envVars map[string]string) error {
		return errors.New("push failed")
	}
	s := newAppService(fs, orch)
	_, err := s.UpdateEnvVars(context.Background(), uuid.New(), map[string]string{"K": "V"})
	require.ErrorContains(t, err, "failed to apply")
}

func TestAppGetStatusAndPods(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusDeploying), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	status, err := s.GetStatus(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "running", status.Phase) // noop returns running

	pods, err := s.GetPods(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, pods)
}

func TestAppGetPodEvents(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	events, err := s.GetPodEvents(context.Background(), uuid.New(), "web-0")
	require.NoError(t, err)
	assert.NotEmpty(t, events)
}

func TestAppRestart(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Restart(context.Background(), uuid.New()))
}

func TestAppRestartBadState(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	require.ErrorContains(t, s.Restart(context.Background(), uuid.New()), "cannot restart")
}

func TestAppRestartOrchError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.RestartFn = func(ctx context.Context, app *model.Application) error { return errors.New("restart failed") }
	s := newAppService(fs, orch)
	require.Error(t, s.Restart(context.Background(), uuid.New()))
}

func TestAppStop(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Stop(context.Background(), uuid.New()))
}

func TestAppStopBadState(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusIdle), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	require.ErrorContains(t, s.Stop(context.Background(), uuid.New()), "cannot stop")
}

func TestAppStopWithHPA(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		a := appWithStatus(model.AppStatusRunning)
		a.Autoscaling = &model.AutoscalingConfig{Enabled: true}
		return a, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Stop(context.Background(), uuid.New()))
}

func TestAppStopHPAError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		a := appWithStatus(model.AppStatusRunning)
		a.Autoscaling = &model.AutoscalingConfig{Enabled: true}
		return a, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeleteHPAFn = func(ctx context.Context, app *model.Application) error { return errors.New("hpa") }
	s := newAppService(fs, orch)
	require.Error(t, s.Stop(context.Background(), uuid.New()))
}

func TestAppStopOrchError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.StopFn = func(ctx context.Context, app *model.Application) error { return errors.New("stop failed") }
	s := newAppService(fs, orch)
	require.Error(t, s.Stop(context.Background(), uuid.New()))
}

func TestAppClearBuildCache(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return appWithStatus(model.AppStatusRunning), nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.ClearBuildCache(context.Background(), uuid.New()))
}

func TestAppWebhookLifecycle(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, GitRepo: "https://github.com/o/r"}, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	cfg, err := s.EnableWebhook(context.Background(), uuid.New(), "https://hef.example.com")
	require.NoError(t, err)
	assert.Contains(t, cfg.WebhookURL, "/webhooks/github/")
	assert.True(t, cfg.AutoDeploy)

	require.NoError(t, s.DisableWebhook(context.Background(), uuid.New()))

	cfg2, err := s.RegenerateWebhookSecret(context.Background(), uuid.New(), "https://hef.example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg2.Secret)

	got, err := s.GetWebhookConfig(context.Background(), uuid.New(), "https://hef.example.com")
	require.NoError(t, err)
	assert.Contains(t, got.WebhookURL, "github")
}

func TestAppWebhookGitLabProvider(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, GitRepo: "https://gitlab.com/o/r"}, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())
	cfg, err := s.GetWebhookConfig(context.Background(), uuid.New(), "https://hef.example.com")
	require.NoError(t, err)
	assert.Contains(t, cfg.WebhookURL, "gitlab")
}

func TestAppSecrets(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, Secrets: map[string]string{"OLD": "v"}}, nil
	}
	s := newAppService(fs, testsupport.NewFakeOrchestrator())

	keys, err := s.GetSecretKeys(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, keys, "OLD")

	keys, err = s.UpdateSecrets(context.Background(), uuid.New(), map[string]string{"NEW": "v", "OLD": ""})
	require.NoError(t, err)
	assert.Contains(t, keys, "NEW")
	assert.NotContains(t, keys, "OLD")
}

func TestAppUpdateSecretsOrchErrorRollsBack(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.EnsureSecretFn = func(ctx context.Context, app *model.Application, secrets map[string]string) error {
		return errors.New("secret failed")
	}
	s := newAppService(fs, orch)
	_, err := s.UpdateSecrets(context.Background(), uuid.New(), map[string]string{"K": "V"})
	require.ErrorContains(t, err, "rolled back")
}
