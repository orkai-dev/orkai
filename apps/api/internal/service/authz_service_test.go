package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const adminRole = string(model.RoleAdmin)

func TestAccessibleProjectIDsAdmin(t *testing.T) {
	fs := testsupport.NewFakeStore()
	a := NewAuthz(fs)

	ids, isAll, err := a.AccessibleProjectIDs(context.Background(), uuid.New(), adminRole, uuid.New())
	require.NoError(t, err)
	assert.True(t, isAll)
	assert.Nil(t, ids)
}

func TestAccessibleProjectIDsTeamListError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return nil, errors.New("db down")
	}
	a := NewAuthz(fs)

	_, isAll, err := a.AccessibleProjectIDs(context.Background(), uuid.New(), "member", uuid.New())
	require.Error(t, err)
	assert.False(t, isAll)
}

func TestAccessibleProjectIDsNoTeams(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return nil, nil
	}
	a := NewAuthz(fs)

	ids, isAll, err := a.AccessibleProjectIDs(context.Background(), uuid.New(), "member", uuid.New())
	require.NoError(t, err)
	assert.False(t, isAll)
	assert.NotNil(t, ids)
	assert.Empty(t, ids)
}

func TestAccessibleProjectIDsListByTeamsError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{uuid.New()}, nil
	}
	fs.ProjectsStore.ListIDsByTeamsFn = func(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID) ([]uuid.UUID, error) {
		return nil, errors.New("boom")
	}
	a := NewAuthz(fs)

	_, _, err := a.AccessibleProjectIDs(context.Background(), uuid.New(), "member", uuid.New())
	require.Error(t, err)
}

func TestAccessibleProjectIDsSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	p1 := uuid.New()
	p2 := uuid.New()
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{uuid.New()}, nil
	}
	fs.ProjectsStore.ListIDsByTeamsFn = func(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{p1, p2}, nil
	}
	a := NewAuthz(fs)

	ids, isAll, err := a.AccessibleProjectIDs(context.Background(), uuid.New(), "member", uuid.New())
	require.NoError(t, err)
	assert.False(t, isAll)
	assert.ElementsMatch(t, []uuid.UUID{p1, p2}, ids)
}

func TestCanAccessProjectGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, errors.New("not found")
	}
	a := NewAuthz(fs)

	ok, err := a.CanAccessProject(context.Background(), uuid.New(), adminRole, uuid.New(), uuid.New())
	require.Error(t, err)
	assert.False(t, ok)
}

func TestCanAccessProjectWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{OrgID: uuid.New()}, nil
	}
	a := NewAuthz(fs)

	ok, err := a.CanAccessProject(context.Background(), uuid.New(), adminRole, uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCanAccessProjectAdmin(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{OrgID: orgID}, nil
	}
	a := NewAuthz(fs)

	ok, err := a.CanAccessProject(context.Background(), uuid.New(), adminRole, orgID, uuid.New())
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanAccessProjectMemberTeamListError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{OrgID: orgID, TeamID: uuid.New()}, nil
	}
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return nil, errors.New("boom")
	}
	a := NewAuthz(fs)

	ok, err := a.CanAccessProject(context.Background(), uuid.New(), "member", orgID, uuid.New())
	require.Error(t, err)
	assert.False(t, ok)
}

func TestCanAccessProjectMemberMatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	teamID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{OrgID: orgID, TeamID: teamID}, nil
	}
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{uuid.New(), teamID}, nil
	}
	a := NewAuthz(fs)

	ok, err := a.CanAccessProject(context.Background(), uuid.New(), "member", orgID, uuid.New())
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanAccessProjectMemberNoMatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{OrgID: orgID, TeamID: uuid.New()}, nil
	}
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{uuid.New()}, nil
	}
	a := NewAuthz(fs)

	ok, err := a.CanAccessProject(context.Background(), uuid.New(), "member", orgID, uuid.New())
	require.NoError(t, err)
	assert.False(t, ok)
}
