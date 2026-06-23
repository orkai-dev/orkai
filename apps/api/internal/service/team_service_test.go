package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTeamService(fs *testsupport.FakeStore) *TeamService {
	jm := auth.NewJWTManager("secret", time.Hour, 24*time.Hour)
	return NewTeamService(fs, jm, testsupport.NewTestLogger(), nil)
}

func TestListMembers(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, p store.ListParams) ([]model.User, int, error) {
		return []model.User{{Email: "a@b.c", Role: model.RoleAdmin}}, 1, nil
	}
	s := newTeamService(fs)
	members, err := s.ListMembers(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "admin", members[0].Role)
}

func TestListMembersError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, p store.ListParams) ([]model.User, int, error) {
		return nil, 0, errors.New("query failed")
	}
	s := newTeamService(fs)
	_, err := s.ListMembers(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestUpdateMemberRoleInvalid(t *testing.T) {
	s := newTeamService(testsupport.NewFakeStore())
	require.Error(t, s.UpdateMemberRole(context.Background(), uuid.New(), uuid.New(), "boss"))
}

func TestUpdateMemberRoleUserNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errors.New("missing")
	}
	s := newTeamService(fs)
	require.Error(t, s.UpdateMemberRole(context.Background(), uuid.New(), uuid.New(), "admin"))
}

func TestUpdateMemberRoleWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: uuid.New()}, nil
	}
	s := newTeamService(fs)
	require.ErrorContains(t, s.UpdateMemberRole(context.Background(), uuid.New(), uuid.New(), "admin"), "not a member")
}

func TestUpdateMemberRoleLastAdmin(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: orgID, Role: model.RoleAdmin}, nil
	}
	fs.UsersStore.CountByRoleFn = func(ctx context.Context, oid uuid.UUID, role string) (int, error) {
		return 1, nil
	}
	s := newTeamService(fs)
	require.ErrorContains(t, s.UpdateMemberRole(context.Background(), orgID, uuid.New(), "member"), "at least one admin")
}

func TestUpdateMemberRoleSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	var updatedRole string
	var keysRevoked bool
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: orgID, Role: model.RoleMember}, nil
	}
	fs.UsersStore.UpdateRoleFn = func(ctx context.Context, userID uuid.UUID, role string) error {
		updatedRole = role
		return nil
	}
	fs.APIKeysStore.RevokeAllForUserFn = func(ctx context.Context, userID uuid.UUID) error {
		keysRevoked = true
		return nil
	}
	s := newTeamService(fs)
	require.NoError(t, s.UpdateMemberRole(context.Background(), orgID, uuid.New(), "admin"))
	assert.Equal(t, "admin", updatedRole)
	assert.True(t, keysRevoked)
}

func TestUpdateMemberRoleRevokeFailure(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: orgID, Role: model.RoleMember}, nil
	}
	fs.APIKeysStore.RevokeAllForUserFn = func(ctx context.Context, userID uuid.UUID) error {
		return errors.New("revoke failed")
	}
	s := newTeamService(fs)
	require.Error(t, s.UpdateMemberRole(context.Background(), orgID, uuid.New(), "admin"))
}

func TestRemoveMemberLastAdmin(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: orgID, Role: model.RoleAdmin}, nil
	}
	fs.UsersStore.CountByRoleFn = func(ctx context.Context, oid uuid.UUID, role string) (int, error) {
		return 1, nil
	}
	s := newTeamService(fs)
	require.ErrorContains(t, s.RemoveMember(context.Background(), orgID, uuid.New()), "last admin")
}

func TestRemoveMemberSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	var removed uuid.UUID
	var keysRevoked bool
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: orgID, Role: model.RoleMember}, nil
	}
	fs.TeamMembersStore.ListByUserFn = func(ctx context.Context, userID uuid.UUID) ([]model.TeamMember, error) {
		return []model.TeamMember{{TeamID: uuid.New()}}, nil
	}
	fs.UsersStore.RemoveFromOrgFn = func(ctx context.Context, userID uuid.UUID) error {
		removed = userID
		return nil
	}
	fs.APIKeysStore.RevokeAllForUserFn = func(ctx context.Context, userID uuid.UUID) error {
		keysRevoked = true
		return nil
	}
	s := newTeamService(fs)
	userID := uuid.New()
	require.NoError(t, s.RemoveMember(context.Background(), orgID, userID))
	assert.Equal(t, userID, removed)
	assert.True(t, keysRevoked)
}

func TestInviteMemberInvalidRole(t *testing.T) {
	s := newTeamService(testsupport.NewFakeStore())
	_, err := s.InviteMember(context.Background(), uuid.New(), uuid.New(), "a@b.c", "boss")
	require.Error(t, err)
}

func TestInviteMemberAlreadyMember(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{OrgID: orgID}, nil
	}
	s := newTeamService(fs)
	_, err := s.InviteMember(context.Background(), orgID, uuid.New(), "a@b.c", "member")
	require.ErrorContains(t, err, "already a member")
}

func TestInviteMemberPendingExists(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.InvitationsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error) {
		return []model.Invitation{{Email: "a@b.c", ExpiresAt: time.Now().Add(time.Hour)}}, nil
	}
	s := newTeamService(fs)
	_, err := s.InviteMember(context.Background(), uuid.New(), uuid.New(), "a@b.c", "member")
	require.ErrorContains(t, err, "already pending")
}

func TestInviteMemberSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	s := newTeamService(fs)
	res, err := s.InviteMember(context.Background(), uuid.New(), uuid.New(), "a@b.c", "member")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Invitation.Token)
	assert.False(t, res.Created)
}

func TestInviteMemberGoogleOnlyCreatesUser(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		switch key {
		case model.SettingAuthGoogleOnly:
			return "true", nil
		case model.SettingGoogleOAuthEnabled:
			return "true", nil
		case model.SettingGoogleOAuthClientID:
			return "client-id", nil
		case model.SettingGoogleOAuthClientSecret:
			return "client-secret", nil
		default:
			return "", nil
		}
	}
	var createdEmail string
	fs.UsersStore.CreateFn = func(ctx context.Context, user *model.User) error {
		createdEmail = user.Email
		user.ID = uuid.New()
		return nil
	}
	s := newTeamService(fs)
	res, err := s.InviteMember(context.Background(), uuid.New(), uuid.New(), "Teammate@Company.com", "member")
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, "teammate@company.com", createdEmail)
	assert.Equal(t, "", res.User.PasswordHash)
}

func TestInviteMemberGoogleOnlyRejectsWhenOAuthDisabled(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		switch key {
		case model.SettingAuthGoogleOnly:
			return "true", nil
		case model.SettingGoogleOAuthEnabled:
			return "false", nil
		default:
			return "", nil
		}
	}
	s := newTeamService(fs)
	_, err := s.InviteMember(context.Background(), uuid.New(), uuid.New(), "user@company.com", "member")
	require.ErrorContains(t, err, "Google OAuth must be enabled")
}

func TestInviteMemberGoogleOnlyRejectsWhenOAuthNotConfigured(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		switch key {
		case model.SettingAuthGoogleOnly:
			return "true", nil
		case model.SettingGoogleOAuthEnabled:
			return "true", nil
		case model.SettingGoogleOAuthClientID:
			return "", nil
		default:
			return "", nil
		}
	}
	s := newTeamService(fs)
	_, err := s.InviteMember(context.Background(), uuid.New(), uuid.New(), "user@company.com", "member")
	require.ErrorContains(t, err, "Google OAuth must be configured")
}

func TestInviteMemberGoogleOnlyRejectsDomain(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		switch key {
		case model.SettingAuthGoogleOnly:
			return "true", nil
		case model.SettingGoogleOAuthEnabled:
			return "true", nil
		case model.SettingGoogleOAuthClientID:
			return "client-id", nil
		case model.SettingGoogleOAuthClientSecret:
			return "client-secret", nil
		case model.SettingOAuthAllowedDomains:
			return "company.com", nil
		default:
			return "", nil
		}
	}
	s := newTeamService(fs)
	_, err := s.InviteMember(context.Background(), uuid.New(), uuid.New(), "user@gmail.com", "member")
	require.ErrorContains(t, err, "email domain is not allowed")
}

func TestAcceptInvitationSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.InvitationsStore.GetByTokenFn = func(ctx context.Context, token string) (*model.Invitation, error) {
		return &model.Invitation{OrgID: orgID, Email: "a@b.c", Role: "member", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{Email: "a@b.c"}, nil
	}
	s := newTeamService(fs)
	require.NoError(t, s.AcceptInvitation(context.Background(), "tok", uuid.New()))
}

func TestAcceptInvitationExpired(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.GetByTokenFn = func(ctx context.Context, token string) (*model.Invitation, error) {
		return &model.Invitation{ExpiresAt: time.Now().Add(-time.Hour)}, nil
	}
	s := newTeamService(fs)
	require.ErrorContains(t, s.AcceptInvitation(context.Background(), "tok", uuid.New()), "expired")
}

func TestAcceptInvitationAlreadyAccepted(t *testing.T) {
	fs := testsupport.NewFakeStore()
	now := time.Now()
	fs.InvitationsStore.GetByTokenFn = func(ctx context.Context, token string) (*model.Invitation, error) {
		return &model.Invitation{ExpiresAt: time.Now().Add(time.Hour), AcceptedAt: &now}, nil
	}
	s := newTeamService(fs)
	require.ErrorContains(t, s.AcceptInvitation(context.Background(), "tok", uuid.New()), "already been accepted")
}

func TestAcceptInvitationEmailMismatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.GetByTokenFn = func(ctx context.Context, token string) (*model.Invitation, error) {
		return &model.Invitation{Email: "a@b.c", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{Email: "x@y.z"}, nil
	}
	s := newTeamService(fs)
	require.ErrorContains(t, s.AcceptInvitation(context.Background(), "tok", uuid.New()), "different email")
}

func TestListAndCancelInvitations(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.ListByOrgFn = func(ctx context.Context, id uuid.UUID) ([]model.Invitation, error) {
		return []model.Invitation{{}}, nil
	}
	fs.InvitationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
		return &model.Invitation{OrgID: orgID, Email: "x@example.com"}, nil
	}
	s := newTeamService(fs)
	invs, err := s.ListInvitations(context.Background(), orgID)
	require.NoError(t, err)
	assert.Len(t, invs, 1)
	require.NoError(t, s.CancelInvitation(context.Background(), orgID, uuid.New()))
}

func TestCancelInvitationNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
		return nil, sql.ErrNoRows
	}
	s := newTeamService(fs)
	err := s.CancelInvitation(context.Background(), uuid.New(), uuid.New())
	require.ErrorContains(t, err, "invitation not found")
}

func TestCancelInvitationWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
		return &model.Invitation{OrgID: uuid.New(), Email: "x@example.com"}, nil
	}
	s := newTeamService(fs)
	err := s.CancelInvitation(context.Background(), uuid.New(), uuid.New())
	require.ErrorContains(t, err, "invitation not found")
}

func TestCreateTeam(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newTeamService(fs)
	team, err := s.CreateTeam(context.Background(), uuid.New(), "  Platform  ", " desc ")
	require.NoError(t, err)
	assert.Equal(t, "Platform", team.Name)
	assert.Equal(t, "desc", team.Description)
}

func TestCreateTeamEmptyName(t *testing.T) {
	s := newTeamService(testsupport.NewFakeStore())
	_, err := s.CreateTeam(context.Background(), uuid.New(), "   ", "")
	require.Error(t, err)
}

func TestListTeams(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.Team, error) {
		return []model.Team{{}}, nil
	}
	s := newTeamService(fs)
	teams, err := s.ListTeams(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, teams, 1)
}

func TestGetTeamWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return &model.Team{OrgID: uuid.New()}, nil
	}
	s := newTeamService(fs)
	_, err := s.GetTeam(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

func TestDeleteTeamWithProjects(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return &model.Team{OrgID: orgID}, nil
	}
	fs.TeamsStore.CountProjectsFn = func(ctx context.Context, teamID uuid.UUID) (int, error) {
		return 2, nil
	}
	s := newTeamService(fs)
	require.ErrorContains(t, s.DeleteTeam(context.Background(), orgID, uuid.New()), "still has projects")
}

func TestDeleteTeamSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return &model.Team{OrgID: orgID}, nil
	}
	s := newTeamService(fs)
	require.NoError(t, s.DeleteTeam(context.Background(), orgID, uuid.New()))
}

func TestAddTeamMember(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return &model.Team{OrgID: orgID}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: orgID}, nil
	}
	s := newTeamService(fs)
	require.NoError(t, s.AddTeamMember(context.Background(), orgID, uuid.New(), uuid.New()))
}

func TestAddTeamMemberWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return &model.Team{OrgID: orgID}, nil
	}
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{OrgID: uuid.New()}, nil
	}
	s := newTeamService(fs)
	require.Error(t, s.AddTeamMember(context.Background(), orgID, uuid.New(), uuid.New()))
}

func TestRemoveTeamMember(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return &model.Team{OrgID: orgID}, nil
	}
	s := newTeamService(fs)
	require.NoError(t, s.RemoveTeamMember(context.Background(), orgID, uuid.New(), uuid.New()))
}

func TestListTeamMembers(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	fs.TeamsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Team, error) {
		return &model.Team{OrgID: orgID}, nil
	}
	fs.TeamMembersStore.ListUsersByTeamFn = func(ctx context.Context, teamID uuid.UUID) ([]model.OrgMember, error) {
		return []model.OrgMember{{}}, nil
	}
	s := newTeamService(fs)
	members, err := s.ListTeamMembers(context.Background(), orgID, uuid.New())
	require.NoError(t, err)
	assert.Len(t, members, 1)
}

func TestListTeamsForUser(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{uuid.New()}, nil
	}
	s := newTeamService(fs)
	ids, err := s.ListTeamsForUser(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, ids, 1)
}

func TestGetInvitationByTokenExpired(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.GetByTokenFn = func(ctx context.Context, token string) (*model.Invitation, error) {
		return &model.Invitation{ExpiresAt: time.Now().Add(-time.Hour)}, nil
	}
	s := newTeamService(fs)
	_, err := s.GetInvitationByToken(context.Background(), "tok")
	require.ErrorContains(t, err, "expired")
}

func TestAcceptInvitationWithRegisterSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.GetByTokenFn = func(ctx context.Context, token string) (*model.Invitation, error) {
		return &model.Invitation{Email: "a@b.c", Role: "member", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("not found")
	}
	s := newTeamService(fs)
	res, err := s.AcceptInvitationWithRegister(context.Background(), "tok", "password123", "Name")
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
}

func TestAcceptInvitationWithRegisterExists(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.GetByTokenFn = func(ctx context.Context, token string) (*model.Invitation, error) {
		return &model.Invitation{Email: "a@b.c", Role: "member", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return &model.User{}, nil
	}
	s := newTeamService(fs)
	_, err := s.AcceptInvitationWithRegister(context.Background(), "tok", "password123", "Name")
	require.ErrorContains(t, err, "already exists")
}
