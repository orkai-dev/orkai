package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

var errBoom = errors.New("boom")

// ─── Cluster: orchestrator error paths ───────────────────────────

func TestClusterGetNodesError(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) { return nil, errBoom }
	h := NewClusterHandler(testsupport.NewFakeTargetRegistry(orch), testsupport.NewFakeStore(), nil)
	r, _, _ := newAuthedRouter()
	r.GET("/nodes", h.GetNodes)
	assert.Equal(t, 500, doJSON(r, "GET", "/nodes", nil).Code)
}

func TestClusterGetEventsError(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	orch.GetClusterEventsFn = func(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) {
		return nil, errBoom
	}
	h := NewClusterHandler(testsupport.NewFakeTargetRegistry(orch), testsupport.NewFakeStore(), nil)
	r, _, _ := newAuthedRouter()
	r.GET("/events", h.GetEvents)
	assert.Equal(t, 500, doJSON(r, "GET", "/events?limit=9999", nil).Code)
}

// ─── Monitoring: store error path ────────────────────────────────

func TestMonitoringSnapshotsError(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	ms.SnapshotsStore.QueryFn = func(ctx context.Context, q store.SnapshotQuery) ([]model.MetricSnapshot, error) {
		return nil, errBoom
	}
	mc := service.NewMetricsCollector(ms, testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry(), testLogger(), nil)
	h := NewMonitoringHandler(mc)
	r, _, _ := newAuthedRouter()
	r.GET("/snapshots", h.GetSnapshots)
	assert.Equal(t, 500, doJSON(r, "GET", "/snapshots", nil).Code)
}

// ─── App: success paths via fakes ────────────────────────────────

func TestAppScaleSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	app := appWithID(id)
	app.Replicas = 1
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) { return app, nil }
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/scale", h.Scale)
	w := doJSON(r, "POST", "/apps/"+id.String()+"/scale", map[string]any{"replicas": 3})
	assert.Equal(t, 200, w.Code)
}

func TestAppRestartSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/restart", h.Restart)
	assert.Equal(t, 200, doJSON(r, "POST", "/apps/"+id.String()+"/restart", nil).Code)
}

func TestAppGetNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return nil, errBoom
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id", h.Get)
	assert.Equal(t, 404, doJSON(r, "GET", "/apps/"+uuid.New().String(), nil).Code)
}

func TestAppGetSecretsSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/secrets", h.GetSecrets)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps/"+id.String()+"/secrets", nil).Code)
}

// ─── Project: list error + service error ─────────────────────────

func TestProjectListError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, p store.ListParams) ([]model.Project, int, error) {
		return nil, 0, errBoom
	}
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.GET("/projects", h.List)
	assert.Equal(t, 500, doJSON(r, "GET", "/projects", nil).Code)
}

// ─── Resource: success paths ─────────────────────────────────────

func TestResourceDeleteSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	uid := uuid.New()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: id}, OrgID: uid}, nil
	}
	r, _, oid := newAuthedRouter()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: id}, OrgID: oid}, nil
	}
	r.DELETE("/resources/:id", newResourceHandler(fs).Delete)
	assert.Equal(t, 204, doJSON(r, "DELETE", "/resources/"+id.String(), nil).Code)
}

// ─── Setting: GetAll error ───────────────────────────────────────

func TestSettingGetAllError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetAllFn = func(ctx context.Context) ([]model.Setting, error) { return nil, errBoom }
	r, _, _ := newAuthedRouter()
	r.GET("/settings", newSettingHandler(fs).GetAll)
	assert.Equal(t, 500, doJSON(r, "GET", "/settings", nil).Code)
}

// ─── Auth: Setup2FA + Me not found ───────────────────────────────

func TestAuthSetup2FA(t *testing.T) {
	fs := testsupport.NewFakeStore()
	uid := uuid.New()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: id}, Email: "a@b.c", TwoFAEnabled: false}, nil
	}
	h := newAuthHandler(fs)
	r := gin.New()
	oid := uuid.New()
	r.Use(authMiddleware(uid, oid, "admin"))
	r.POST("/auth/2fa/setup", h.Setup2FA)
	assert.Equal(t, 200, doJSON(r, "POST", "/auth/2fa/setup", nil).Code)
}

func TestAuthMeNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return nil, errBoom
	}
	h := newAuthHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/auth/me", h.Me)
	assert.Equal(t, 404, doJSON(r, "GET", "/auth/me", nil).Code)
}
