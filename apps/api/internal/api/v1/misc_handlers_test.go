package v1

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

// ─── NodeHandler ─────────────────────────────────────────────────

func newNodeHandler(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *NodeHandler {
	return NewNodeHandler(service.NewNodeService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), nil))
}

func TestNodeList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ServerNodesStore.ListFn = func(ctx context.Context) ([]model.ServerNode, error) {
		return []model.ServerNode{{}}, nil
	}
	h := newNodeHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/nodes", h.List)
	assert.Equal(t, 200, doJSON(r, "GET", "/nodes", nil).Code)
}

func TestNodeCreateValidation(t *testing.T) {
	h := newNodeHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/nodes", h.Create)
	assert.Equal(t, 400, doJSON(r, "POST", "/nodes", map[string]any{}).Code)
}

func TestNodeInitializeBadID(t *testing.T) {
	h := newNodeHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/nodes/:id/init", h.Initialize)
	assert.Equal(t, 400, doJSON(r, "POST", "/nodes/bad/init", nil).Code)
}

func TestNodeDeleteBadID(t *testing.T) {
	h := newNodeHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.DELETE("/nodes/:id", h.Delete)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/nodes/bad", nil).Code)
}

// ─── SystemBackupHandler ─────────────────────────────────────────

func newSystemBackupHandler(fs *testsupport.FakeStore) *SystemBackupHandler {
	settings := service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(), testLogger())
	svc := service.NewSystemBackupService(fs, settings, "postgres://u:p@localhost:5432/db", testLogger(), testsupport.NewFakeEnqueuer(), testsupport.NewProviders(fs))
	return NewSystemBackupHandler(svc)
}

func TestSystemBackupGetConfig(t *testing.T) {
	h := newSystemBackupHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/system/backup/config", h.GetConfig)
	assert.Equal(t, 200, doJSON(r, "GET", "/system/backup/config", nil).Code)
}

func TestSystemBackupSaveConfigDisabled(t *testing.T) {
	h := newSystemBackupHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.PUT("/system/backup/config", h.SaveConfig)
	// disabled config saves fine
	assert.Equal(t, 200, doJSON(r, "PUT", "/system/backup/config", map[string]any{"enabled": false}).Code)
}

func TestSystemBackupListBackups(t *testing.T) {
	h := newSystemBackupHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/system/backups", h.ListBackups)
	assert.Equal(t, 200, doJSON(r, "GET", "/system/backups", nil).Code)
}

func TestSystemBackupScanS3Validation(t *testing.T) {
	h := newSystemBackupHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/system/backups/scan", h.ScanS3Backups)
	assert.Equal(t, 400, doJSON(r, "POST", "/system/backups/scan", map[string]any{}).Code)
}

func TestSystemBackupRestoreS3Validation(t *testing.T) {
	h := newSystemBackupHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/system/backups/restore", h.RestoreFromS3)
	assert.Equal(t, 400, doJSON(r, "POST", "/system/backups/restore", map[string]any{}).Code)
}

// ─── NotificationHandler ─────────────────────────────────────────

func newNotificationHandler(fs *testsupport.FakeStore) *NotificationHandler {
	settings := service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(), testLogger())
	return NewNotificationHandler(service.NewNotificationService(fs, settings, testLogger()))
}

func TestNotificationListChannels(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
		return nil, nil
	}
	h := newNotificationHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/notifications/channels", h.ListChannels)
	assert.Equal(t, 200, doJSON(r, "GET", "/notifications/channels", nil).Code)
}

func TestNotificationSaveChannelValidation(t *testing.T) {
	h := newNotificationHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/notifications/channels", h.SaveChannel)
	assert.Equal(t, 400, doJSON(r, "POST", "/notifications/channels", map[string]any{}).Code)
}

func TestNotificationTestChannelValidation(t *testing.T) {
	h := newNotificationHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/notifications/channels/test", h.TestChannel)
	assert.Equal(t, 400, doJSON(r, "POST", "/notifications/channels/test", map[string]any{}).Code)
}

func TestNotificationGetSMTPConfig(t *testing.T) {
	h := newNotificationHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/notifications/smtp", h.GetSMTPConfig)
	assert.Equal(t, 200, doJSON(r, "GET", "/notifications/smtp", nil).Code)
}

func TestNotificationSaveSMTPValidation(t *testing.T) {
	h := newNotificationHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.PUT("/notifications/smtp", h.SaveSMTPConfig)
	assert.Equal(t, 400, doJSON(r, "PUT", "/notifications/smtp", "not-json").Code)
}

func TestNotificationTestSMTP(t *testing.T) {
	h := newNotificationHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/notifications/smtp/test", h.TestSMTP)
	// SMTP not configured → 400
	assert.Equal(t, 400, doJSON(r, "POST", "/notifications/smtp/test", nil).Code)
}

// ─── AuthHandler ─────────────────────────────────────────────────

func newAuthHandler(fs *testsupport.FakeStore) *AuthHandler {
	return NewAuthHandler(service.NewAuthService(fs, testJWT(), testLogger()), nil)
}

func TestAuthSetupStatus(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 1, nil }
	h := newAuthHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/auth/setup", h.SetupStatus)
	assert.Equal(t, 200, doJSON(r, "GET", "/auth/setup", nil).Code)
}

func TestAuthRegisterValidation(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/auth/register", h.Register)
	assert.Equal(t, 400, doJSON(r, "POST", "/auth/register", map[string]any{}).Code)
}

func TestAuthLoginValidation(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/auth/login", h.Login)
	assert.Equal(t, 400, doJSON(r, "POST", "/auth/login", map[string]any{}).Code)
}

func TestAuthLoginBadCreds(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByEmailFn = func(ctx context.Context, email string) (*model.User, error) {
		return nil, context.Canceled
	}
	h := newAuthHandler(fs)
	r, _, _ := newAuthedRouter()
	r.POST("/auth/login", h.Login)
	assert.Equal(t, 401, doJSON(r, "POST", "/auth/login", map[string]any{"email": "a@b.c", "password": "secret12"}).Code)
}

func TestAuthRefreshValidation(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/auth/refresh", h.Refresh)
	assert.Equal(t, 400, doJSON(r, "POST", "/auth/refresh", map[string]any{}).Code)
}

func TestAuthRefreshBadToken(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/auth/refresh", h.Refresh)
	assert.Equal(t, 401, doJSON(r, "POST", "/auth/refresh", map[string]any{"refresh_token": "bad"}).Code)
}

func TestAuthMe(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: id}}, nil
	}
	h := newAuthHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/auth/me", h.Me)
	assert.Equal(t, 200, doJSON(r, "GET", "/auth/me", nil).Code)
}

func TestAuthListAvatars(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/auth/avatars", h.ListAvatars)
	assert.Equal(t, 200, doJSON(r, "GET", "/auth/avatars", nil).Code)
}

func TestAuthUpdateProfileValidation(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.PUT("/auth/profile", h.UpdateProfile)
	assert.Equal(t, 400, doJSON(r, "PUT", "/auth/profile", "bad").Code)
}

func TestAuthChangePasswordValidation(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/auth/password", h.ChangePassword)
	assert.Equal(t, 400, doJSON(r, "POST", "/auth/password", map[string]any{}).Code)
}

func TestAuth2FAEndpointsValidation(t *testing.T) {
	h := newAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/auth/2fa/verify", h.Verify2FA)
	r.POST("/auth/2fa/disable", h.Disable2FA)
	assert.Equal(t, 400, doJSON(r, "POST", "/auth/2fa/verify", map[string]any{}).Code)
	assert.Equal(t, 400, doJSON(r, "POST", "/auth/2fa/disable", map[string]any{}).Code)
}

// ─── TeamHandler ─────────────────────────────────────────────────

func newTeamHandler(fs *testsupport.FakeStore) *TeamHandler {
	settings := service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(), testLogger())
	notif := service.NewNotificationService(fs, settings, testLogger())
	return NewTeamHandler(service.NewTeamService(fs, testJWT(), testLogger(), notif), notif, "http://localhost", nil)
}

func TestTeamListMembers(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := newTeamHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/team/members", h.ListMembers)
	assert.Equal(t, 200, doJSON(r, "GET", "/team/members", nil).Code)
}

func TestTeamUpdateMemberRoleBadID(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.PUT("/team/members/:id", h.UpdateMemberRole)
	assert.Equal(t, 400, doJSON(r, "PUT", "/team/members/bad", map[string]any{"role": "member"}).Code)
}

func TestTeamUpdateMemberRoleSelf(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	uid, oid := uuid.New(), uuid.New()
	r := gin.New()
	r.Use(authMiddleware(uid, oid, "admin"))
	r.PUT("/team/members/:id", h.UpdateMemberRole)
	assert.Equal(t, 400, doJSON(r, "PUT", "/team/members/"+uid.String(), map[string]any{"role": "member"}).Code)
}

func TestTeamRemoveMemberBadID(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.DELETE("/team/members/:id", h.RemoveMember)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/team/members/bad", nil).Code)
}

func TestTeamInviteMemberValidation(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/team/invitations", h.InviteMember)
	assert.Equal(t, 400, doJSON(r, "POST", "/team/invitations", map[string]any{}).Code)
}

func TestTeamAcceptInvitationValidation(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/team/invitations/accept", h.AcceptInvitation)
	assert.Equal(t, 400, doJSON(r, "POST", "/team/invitations/accept", map[string]any{}).Code)
}

func TestTeamListInvitations(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.InvitationsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error) {
		return nil, nil
	}
	h := newTeamHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/team/invitations", h.ListInvitations)
	assert.Equal(t, 200, doJSON(r, "GET", "/team/invitations", nil).Code)
}

func TestTeamCancelInvitationBadID(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.DELETE("/team/invitations/:id", h.CancelInvitation)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/team/invitations/bad", nil).Code)
}

func TestTeamListTeams(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TeamsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.Team, error) {
		return nil, nil
	}
	h := newTeamHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/teams", h.ListTeams)
	assert.Equal(t, 200, doJSON(r, "GET", "/teams", nil).Code)
}

func TestTeamCreateTeamValidation(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/teams", h.CreateTeam)
	assert.Equal(t, 400, doJSON(r, "POST", "/teams", map[string]any{}).Code)
}

func TestTeamDeleteTeamBadID(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.DELETE("/teams/:id", h.DeleteTeam)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/teams/bad", nil).Code)
}

func TestTeamListTeamMembersBadID(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/teams/:id/members", h.ListTeamMembers)
	assert.Equal(t, 400, doJSON(r, "GET", "/teams/bad/members", nil).Code)
}

func TestTeamAddTeamMemberBadID(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/teams/:id/members", h.AddTeamMember)
	assert.Equal(t, 400, doJSON(r, "POST", "/teams/bad/members", map[string]any{"user_id": uuid.New().String()}).Code)
}

func TestTeamRemoveTeamMemberBadIDs(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.DELETE("/teams/:id/members/:userId", h.RemoveTeamMember)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/teams/bad/members/"+uuid.New().String(), nil).Code)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/teams/"+uuid.New().String()+"/members/bad", nil).Code)
}

func TestTeamGetInvitationByTokenMissing(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/invitations", h.GetInvitationByToken)
	assert.Equal(t, 400, doJSON(r, "GET", "/invitations", nil).Code)
}

func TestTeamAcceptInvitationPublicValidation(t *testing.T) {
	h := newTeamHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.POST("/invitations/accept", h.AcceptInvitationPublic)
	assert.Equal(t, 400, doJSON(r, "POST", "/invitations/accept", map[string]any{}).Code)
}

// ─── Guards ──────────────────────────────────────────────────────

func TestGuardsProjectBadID(t *testing.T) {
	g := NewGuards(testsupport.NewFakeStore(), service.NewAuthz(testsupport.NewFakeStore()))
	r, _, _ := newAuthedRouter()
	r.GET("/p/:id", g.Project("id"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 400, doJSON(r, "GET", "/p/bad", nil).Code)
}

func TestGuardsProjectAllowed(t *testing.T) {
	fs := testsupport.NewFakeStore()
	g := NewGuards(fs, service.NewAuthz(fs))
	r, _, oid := newAuthedRouter() // admin → allowed
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: id}, OrgID: oid}, nil
	}
	r.GET("/p/:id", g.Project("id"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doJSON(r, "GET", "/p/"+uuid.New().String(), nil).Code)
}

func TestGuardsAppNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return nil, context.Canceled
	}
	g := NewGuards(fs, service.NewAuthz(fs))
	r, _, _ := newAuthedRouter()
	r.GET("/a/:id", g.App("id"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 404, doJSON(r, "GET", "/a/"+uuid.New().String(), nil).Code)
}

func TestGuardsAppAllowed(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{ProjectID: uuid.New()}, nil
	}
	g := NewGuards(fs, service.NewAuthz(fs))
	r, _, oid := newAuthedRouter()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: id}, OrgID: oid}, nil
	}
	r.GET("/a/:id", g.App("id"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 200, doJSON(r, "GET", "/a/"+uuid.New().String(), nil).Code)
}

func TestGuardsDatabaseAndCronAndDeployAndDomainBadID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	g := NewGuards(fs, service.NewAuthz(fs))
	r, _, _ := newAuthedRouter()
	r.GET("/db/:id", g.Database("id"), func(c *gin.Context) { c.Status(200) })
	r.GET("/cj/:id", g.CronJob("id"), func(c *gin.Context) { c.Status(200) })
	r.GET("/dep/:id", g.Deployment("id"), func(c *gin.Context) { c.Status(200) })
	r.GET("/dom/:id", g.Domain("id"), func(c *gin.Context) { c.Status(200) })
	assert.Equal(t, 400, doJSON(r, "GET", "/db/bad", nil).Code)
	assert.Equal(t, 400, doJSON(r, "GET", "/cj/bad", nil).Code)
	assert.Equal(t, 400, doJSON(r, "GET", "/dep/bad", nil).Code)
	assert.Equal(t, 400, doJSON(r, "GET", "/dom/bad", nil).Code)
}
