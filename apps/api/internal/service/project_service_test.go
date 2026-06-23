package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProjectService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *ProjectService {
	return NewProjectService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewTestLogger(), nil)
}

func TestGenerateNamespace(t *testing.T) {
	id := uuid.MustParse("12345678-1234-1234-1234-1234567890ab")

	ns := generateNamespace("My Project", id)
	assert.True(t, strings.HasPrefix(ns, "sb-my-project-"))

	// Non-ASCII names fall back to "proj".
	nsNonASCII := generateNamespace("日本語", id)
	assert.True(t, strings.HasPrefix(nsNonASCII, "sb-proj-"))

	// Special characters are stripped and hyphens collapsed.
	nsSpecial := generateNamespace("a@@b__c  d", id)
	assert.NotContains(t, nsSpecial, "--")
	assert.NotContains(t, nsSpecial, "@")

	// Very long names are truncated to 63 chars.
	long := strings.Repeat("verylongname", 10)
	nsLong := generateNamespace(long, id)
	assert.LessOrEqual(t, len(nsLong), 63)
}

func validTeam(orgID uuid.UUID) *model.Team {
	return &model.Team{BaseModel: model.BaseModel{ID: uuid.New()}, OrgID: orgID}
}

func TestProjectCreateInvalidEnvironment(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateProjectInput{Environment: "bogus", TeamID: uuid.New()})
	require.ErrorContains(t, err, "invalid environment")
}

func TestProjectCreateTeamNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return nil, errors.New("missing")
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateProjectInput{Environment: model.EnvProd, TeamID: uuid.New()})
	require.ErrorContains(t, err, "team not found")
}

func TestProjectCreateTeamWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return validTeam(uuid.New()), nil // different org
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateProjectInput{Environment: model.EnvProd, TeamID: uuid.New()})
	require.ErrorContains(t, err, "team not found")
}

func TestProjectCreateStoreError(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return validTeam(orgID), nil
	}
	fs.ProjectsStore.CreateFn = func(ctx context.Context, project *model.Project) error {
		return errors.New("insert failed")
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), orgID, CreateProjectInput{Name: "p", Environment: model.EnvProd, TeamID: uuid.New()})
	require.Error(t, err)
}

func TestProjectCreateNamespaceFailureRollsBack(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return validTeam(orgID), nil
	}
	fs.ProjectsStore.CreateFn = func(ctx context.Context, project *model.Project) error {
		project.ID = uuid.New()
		return nil
	}
	deleted := false
	fs.ProjectsStore.DeleteFn = func(ctx context.Context, id uuid.UUID) error {
		deleted = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.CreateNamespaceFn = func(ctx context.Context, name string) error {
		return errors.New("ns failed")
	}
	s := newProjectService(fs, orch)
	_, err := s.Create(context.Background(), orgID, CreateProjectInput{Name: "p", Environment: model.EnvProd, TeamID: uuid.New()})
	require.ErrorContains(t, err, "failed to create namespace")
	assert.True(t, deleted)
}

func TestProjectCreateServiceAccountFailureRollsBack(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return validTeam(orgID), nil
	}
	fs.ProjectsStore.CreateFn = func(ctx context.Context, project *model.Project) error {
		project.ID = uuid.New()
		return nil
	}
	deleted := false
	fs.ProjectsStore.DeleteFn = func(ctx context.Context, id uuid.UUID) error {
		deleted = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.EnsureServiceAccountFn = func(ctx context.Context, namespace, name string) error {
		return errors.New("sa failed")
	}
	s := newProjectService(fs, orch)
	_, err := s.Create(context.Background(), orgID, CreateProjectInput{Name: "p", Environment: model.EnvProd, TeamID: uuid.New()})
	require.ErrorContains(t, err, "failed to create service account")
	assert.True(t, deleted)
}

func TestProjectCreateSuccess(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return validTeam(orgID), nil
	}
	fs.ProjectsStore.CreateFn = func(ctx context.Context, project *model.Project) error {
		project.ID = uuid.New()
		return nil
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	p, err := s.Create(context.Background(), orgID, CreateProjectInput{Name: "Cool App", Environment: model.EnvProd, TeamID: uuid.New()})
	require.NoError(t, err)
	assert.NotEmpty(t, p.Namespace)
	assert.NotEmpty(t, p.ServiceAccount)
}

func TestProjectGetAndList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	pid := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: pid}}, nil
	}
	fs.ProjectsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
		return []model.Project{{}}, 1, nil
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())

	got, err := s.GetByID(context.Background(), pid)
	require.NoError(t, err)
	assert.Equal(t, pid, got.ID)

	list, total, err := s.List(context.Background(), uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, list, 1)
}

func TestProjectListForUserAdmin(t *testing.T) {
	fs := testsupport.NewFakeStore()
	called := false
	fs.ProjectsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
		called = true
		return nil, 0, nil
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, _, err := s.ListForUser(context.Background(), uuid.New(), adminRole, uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.True(t, called)
}

func TestProjectListForUserMemberTeamError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return nil, errors.New("boom")
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, _, err := s.ListForUser(context.Background(), uuid.New(), "member", uuid.New(), store.DefaultListParams())
	require.Error(t, err)
}

func TestProjectListForUserMember(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{uuid.New()}, nil
	}
	called := false
	fs.ProjectsStore.ListByTeamsFn = func(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
		called = true
		return nil, 0, nil
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, _, err := s.ListForUser(context.Background(), uuid.New(), "member", uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.True(t, called)
}

func TestProjectUpdateGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, errors.New("not found")
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), UpdateProjectInput{})
	require.Error(t, err)
}

func TestProjectUpdateInvalidEnvironment(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	bad := model.Environment("nope")
	_, err := s.Update(context.Background(), uuid.New(), UpdateProjectInput{Environment: &bad})
	require.ErrorContains(t, err, "invalid environment")
}

func TestProjectUpdateFull(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{
			BaseModel: model.BaseModel{ID: uuid.New()},
			Namespace: "sb-test-1234",
		}, nil
	}
	fs.ApplicationsStore.ListByProjectFn = func(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Application, int, error) {
		return []model.Application{{Name: "app1", EnvVars: map[string]string{"APP": "v"}}}, 1, nil
	}
	fs.CronJobsStore.ListByProjectFn = func(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.CronJob, int, error) {
		return []model.CronJob{{Name: "cj1", EnvVars: map[string]string{"CJ": "v"}}}, 1, nil
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())

	name := "Renamed"
	desc := "new desc"
	env := model.EnvTesting
	netPol := true
	_, err := s.Update(context.Background(), uuid.New(), UpdateProjectInput{
		Name:                 &name,
		Description:          &desc,
		Environment:          &env,
		EnvVars:              map[string]string{"GLOBAL": "1"},
		ResourceQuota:        &model.ResourceQuotaConfig{CPULimit: "1000m"},
		NetworkPolicyEnabled: &netPol,
	})
	require.NoError(t, err)
}

func TestProjectUpdateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	fs.ProjectsStore.UpdateFn = func(ctx context.Context, project *model.Project) error {
		return errors.New("update failed")
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Update(context.Background(), uuid.New(), UpdateProjectInput{})
	require.Error(t, err)
}

func TestProjectUpdateEnvVars(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.UpdateEnvVars(context.Background(), uuid.New(), map[string]string{"K": "V"})
	require.NoError(t, err)
}

func TestProjectDeleteGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, errors.New("missing")
	}
	s := newProjectService(fs, testsupport.NewFakeOrchestrator())
	require.Error(t, s.Delete(context.Background(), uuid.New()))
}

func TestProjectDeleteSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "sb-test-1234"}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	// Even if namespace delete errors, it is logged and the DB delete proceeds.
	orch.DeleteNamespaceFn = func(ctx context.Context, name string) error {
		return errors.New("ns delete failed")
	}
	s := newProjectService(fs, orch)
	require.NoError(t, s.Delete(context.Background(), uuid.New()))
}
