package v1

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
)

type guardCase struct {
	name   string
	method string
	path   string
	mount  func(r gin.IRoutes, g *Guards, ok func(*gin.Context))
}

func newMemberRouter(fs *testsupport.FakeStore, userID, orgID uuid.UUID) *gin.Engine {
	r := gin.New()
	r.Use(authMiddleware(userID, orgID, string(model.RoleMember)))
	return r
}

func wireMemberTeams(fs *testsupport.FakeStore, userID uuid.UUID, teamIDs ...uuid.UUID) {
	fs.TeamMembersStore.ListTeamIDsByUserFn = func(_ context.Context, uid uuid.UUID) ([]uuid.UUID, error) {
		if uid == userID {
			return teamIDs, nil
		}
		return nil, nil
	}
}

func wireProject(fs *testsupport.FakeStore, orgID, teamID, projectID uuid.UUID) {
	fs.ProjectsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{
			BaseModel: model.BaseModel{ID: projectID},
			OrgID:     orgID,
			TeamID:    teamID,
		}, nil
	}
}

func TestRBACMatrixMemberDeniedAdminAllowed(t *testing.T) {
	orgID := uuid.New()
	teamID := uuid.New()
	projectID := uuid.New()
	userID := uuid.New()

	fs := testsupport.NewFakeStore()
	authz := service.NewAuthz(fs)
	g := NewGuards(fs, authz)
	wireProject(fs, orgID, teamID, projectID)
	wireMemberTeams(fs, userID, teamID)

	appID := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: appID}, ProjectID: projectID}, nil
	}

	dbID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: dbID}, ProjectID: projectID}, nil
	}

	cjID := uuid.New()
	fs.CronJobsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{BaseModel: model.BaseModel{ID: cjID}, ProjectID: projectID}, nil
	}

	depID := uuid.New()
	fs.DeploymentsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.Deployment, error) {
		return &model.Deployment{BaseModel: model.BaseModel{ID: depID}, AppID: appID}, nil
	}

	domID := uuid.New()
	fs.DomainsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.Domain, error) {
		return &model.Domain{BaseModel: model.BaseModel{ID: domID}, AppID: appID}, nil
	}

	cases := []guardCase{
		{
			name:   "project",
			method: http.MethodGet,
			path:   "/projects/" + projectID.String(),
			mount: func(r gin.IRoutes, g *Guards, ok func(*gin.Context)) {
				r.GET("/projects/:id", g.Project("id"), ok)
			},
		},
		{
			name:   "app",
			method: http.MethodGet,
			path:   "/apps/" + appID.String(),
			mount: func(r gin.IRoutes, g *Guards, ok func(*gin.Context)) {
				r.GET("/apps/:id", g.App("id"), ok)
			},
		},
		{
			name:   "database",
			method: http.MethodGet,
			path:   "/databases/" + dbID.String(),
			mount: func(r gin.IRoutes, g *Guards, ok func(*gin.Context)) {
				r.GET("/databases/:id", g.Database("id"), ok)
			},
		},
		{
			name:   "cronjob",
			method: http.MethodGet,
			path:   "/cronjobs/" + cjID.String(),
			mount: func(r gin.IRoutes, g *Guards, ok func(*gin.Context)) {
				r.GET("/cronjobs/:id", g.CronJob("id"), ok)
			},
		},
		{
			name:   "deployment",
			method: http.MethodGet,
			path:   "/deployments/" + depID.String(),
			mount: func(r gin.IRoutes, g *Guards, ok func(*gin.Context)) {
				r.GET("/deployments/:id", g.Deployment("id"), ok)
			},
		},
		{
			name:   "domain",
			method: http.MethodGet,
			path:   "/domains/" + domID.String(),
			mount: func(r gin.IRoutes, g *Guards, ok func(*gin.Context)) {
				r.GET("/domains/:id", g.Domain("id"), ok)
			},
		},
	}

	okHandler := func(c *gin.Context) { c.Status(http.StatusOK) }

	for _, tc := range cases {
		t.Run(tc.name+" member allowed", func(t *testing.T) {
			r := newMemberRouter(fs, userID, orgID)
			tc.mount(r, g, okHandler)
			assert.Equal(t, http.StatusOK, doJSON(r, tc.method, tc.path, nil).Code)
		})
	}

	_, _, adminOrg := newAuthedRouter()
	fs.ProjectsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: adminOrg, TeamID: uuid.New()}, nil
	}
	for _, tc := range cases {
		t.Run(tc.name+" admin allowed", func(t *testing.T) {
			r := gin.New()
			r.Use(authMiddleware(uuid.New(), adminOrg, string(model.RoleAdmin)))
			tc.mount(r, g, okHandler)
			assert.Equal(t, http.StatusOK, doJSON(r, tc.method, tc.path, nil).Code)
		})
	}
}

func TestRBACMatrixMemberNotOnTeamDenied(t *testing.T) {
	orgID := uuid.New()
	projectID := uuid.New()
	userID := uuid.New()
	otherTeam := uuid.New()

	fs := testsupport.NewFakeStore()
	authz := service.NewAuthz(fs)
	g := NewGuards(fs, authz)
	wireProject(fs, orgID, otherTeam, projectID)
	wireMemberTeams(fs, userID, uuid.New())

	r := newMemberRouter(fs, userID, orgID)
	r.GET("/projects/:id", g.Project("id"), func(c *gin.Context) { c.Status(http.StatusOK) })
	assert.Equal(t, http.StatusForbidden, doJSON(r, http.MethodGet, "/projects/"+projectID.String(), nil).Code)
}

func TestRequireRoleMemberDeniedOnAdminRoute(t *testing.T) {
	r := gin.New()
	r.Use(authMiddleware(uuid.New(), uuid.New(), string(model.RoleMember)))
	r.GET("/admin-only", middleware.RequireRole(string(model.RoleAdmin)), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	assert.Equal(t, http.StatusForbidden, doJSON(r, http.MethodGet, "/admin-only", nil).Code)
}

func TestRequireRoleAdminAllowed(t *testing.T) {
	r := gin.New()
	r.Use(authMiddleware(uuid.New(), uuid.New(), string(model.RoleAdmin)))
	r.GET("/admin-only", middleware.RequireRole(string(model.RoleAdmin)), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	require.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/admin-only", nil).Code)
}
