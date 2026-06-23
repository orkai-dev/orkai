package v1

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

// ─── CronJobHandler ──────────────────────────────────────────────

func newCronJobHandler(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *CronJobHandler {
	return NewCronJobHandler(service.NewCronJobService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), nil), service.NewAuthz(fs))
}

func TestCronListByProjectBadID(t *testing.T) {
	h := newCronJobHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id/cronjobs", h.ListByProject)
	assert.Equal(t, 400, doJSON(r, "GET", "/projects/bad/cronjobs", nil).Code)
}

func TestCronListByProject(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.CronJobsStore.ListByProjectFn = func(ctx context.Context, pid uuid.UUID, p store.ListParams) ([]model.CronJob, int, error) {
		return nil, 0, nil
	}
	h := newCronJobHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id/cronjobs", h.ListByProject)
	assert.Equal(t, 200, doJSON(r, "GET", "/projects/"+uuid.New().String()+"/cronjobs", nil).Code)
}

func TestCronCreateValidation(t *testing.T) {
	h := newCronJobHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/cronjobs", h.Create)
	assert.Equal(t, 400, doJSON(r, "POST", "/cronjobs", map[string]any{}).Code)
}

func TestCronGetBadID(t *testing.T) {
	h := newCronJobHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/cronjobs/:id", h.Get)
	assert.Equal(t, 400, doJSON(r, "GET", "/cronjobs/bad", nil).Code)
}

func TestCronGetSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.CronJobsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.CronJob, error) {
		return &model.CronJob{BaseModel: model.BaseModel{ID: id}}, nil
	}
	h := newCronJobHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/cronjobs/:id", h.Get)
	assert.Equal(t, 200, doJSON(r, "GET", "/cronjobs/"+id.String(), nil).Code)
}

func TestCronUpdateBadID(t *testing.T) {
	h := newCronJobHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.PUT("/cronjobs/:id", h.Update)
	assert.Equal(t, 400, doJSON(r, "PUT", "/cronjobs/bad", map[string]any{}).Code)
}

func TestCronDeleteBadID(t *testing.T) {
	h := newCronJobHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.DELETE("/cronjobs/:id", h.Delete)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/cronjobs/bad", nil).Code)
}

func TestCronTriggerBadID(t *testing.T) {
	h := newCronJobHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/cronjobs/:id/trigger", h.Trigger)
	assert.Equal(t, 400, doJSON(r, "POST", "/cronjobs/bad/trigger", nil).Code)
}

func TestCronListRunsBadID(t *testing.T) {
	h := newCronJobHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/cronjobs/:id/runs", h.ListRuns)
	assert.Equal(t, 400, doJSON(r, "GET", "/cronjobs/bad/runs", nil).Code)
}

// ─── DomainHandler ───────────────────────────────────────────────

func newDomainHandler(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *DomainHandler {
	ds := service.NewDomainService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger()), nil)
	return NewDomainHandler(ds, fs)
}

// wireAppOrg makes verifyAppOrg succeed for the given org.
func wireAppOrg(fs *testsupport.FakeStore, orgID uuid.UUID) (appID uuid.UUID) {
	appID = uuid.New()
	projID := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: appID}, ProjectID: projID}, nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projID}, OrgID: orgID}, nil
	}
	return appID
}

func TestDomainListByAppBadID(t *testing.T) {
	h := newDomainHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/domains", h.ListByApp)
	assert.Equal(t, 400, doJSON(r, "GET", "/apps/bad/domains", nil).Code)
}

func TestDomainListByAppForbidden(t *testing.T) {
	fs := testsupport.NewFakeStore()
	wireAppOrg(fs, uuid.New()) // different org than router's
	h := newDomainHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/domains", h.ListByApp)
	assert.Equal(t, 403, doJSON(r, "GET", "/apps/"+uuid.New().String()+"/domains", nil).Code)
}

func TestDomainListByAppSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	r := gin.New()
	uid, oid := uuid.New(), uuid.New()
	r.Use(authMiddleware(uid, oid, "admin"))
	appID := wireAppOrg(fs, oid)
	fs.DomainsStore.ListByAppFn = func(ctx context.Context, _ uuid.UUID) ([]model.Domain, error) {
		return []model.Domain{{}}, nil
	}
	h := newDomainHandler(fs, testsupport.NewFakeOrchestrator())
	r.GET("/apps/:id/domains", h.ListByApp)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps/"+appID.String()+"/domains", nil).Code)
}

func TestDomainCreateBadID(t *testing.T) {
	h := newDomainHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/domains", h.Create)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/domains", map[string]any{}).Code)
}

func TestDomainDeleteBadID(t *testing.T) {
	h := newDomainHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.DELETE("/domains/:id", h.Delete)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/domains/bad", nil).Code)
}

func TestDomainUpdateBadID(t *testing.T) {
	h := newDomainHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.PUT("/domains/:id", h.Update)
	assert.Equal(t, 400, doJSON(r, "PUT", "/domains/bad", map[string]any{}).Code)
}

func TestDomainGenerateBadID(t *testing.T) {
	h := newDomainHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/domains/generate", h.Generate)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/domains/generate", nil).Code)
}
