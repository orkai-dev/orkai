package v1

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

func newAppHandler(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *AppHandler {
	ra := service.NewRegistryAuth(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testLogger())
	ds := service.NewDomainService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger()), nil)
	appSvc := service.NewAppService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), ds, ra, nil)
	mc := service.NewMetricsCollector(testsupport.NewFakeMetricsStore(), fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), nil)
	return NewAppHandler(appSvc, mc, fs, service.NewAuthz(fs))
}

func TestParseMillicores(t *testing.T) {
	assert.Equal(t, 0.0, parseMillicores(""))
	assert.Equal(t, 250.0, parseMillicores("250m"))
	assert.Equal(t, 1000.0, parseMillicores("1"))
	assert.InDelta(t, 1.0, parseMillicores("1000000n"), 0.001)
}

func TestParseMiB(t *testing.T) {
	assert.Equal(t, 0.0, parseMiB(""))
	assert.Equal(t, 1.0, parseMiB("1Mi"))
	assert.Equal(t, 1024.0, parseMiB("1Gi"))
	assert.InDelta(t, 0.5, parseMiB("512Ki"), 0.001)
}

func TestAppListAll(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.AppListFilter) ([]model.Application, int, error) {
		return []model.Application{{}}, 1, nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps", h.ListAll)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps", nil).Code)
}

func TestAppListByProjectBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id/apps", h.ListByProject)
	assert.Equal(t, 400, doJSON(r, "GET", "/projects/bad/apps", nil).Code)
}

func TestAppListByProject(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.ListByProjectFn = func(ctx context.Context, pid uuid.UUID, p store.ListParams) ([]model.Application, int, error) {
		return []model.Application{{}}, 1, nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id/apps", h.ListByProject)
	assert.Equal(t, 200, doJSON(r, "GET", "/projects/"+uuid.New().String()+"/apps", nil).Code)
}

func TestAppCreateValidation(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps", h.Create)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps", map[string]any{}).Code)
}

func TestAppGetBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id", h.Get)
	assert.Equal(t, 400, doJSON(r, "GET", "/apps/bad", nil).Code)
}

func appWithID(id uuid.UUID) *model.Application {
	return &model.Application{BaseModel: model.BaseModel{ID: id}, Name: "web", Status: model.AppStatusRunning}
}

func TestAppGetSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id", h.Get)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps/"+id.String(), nil).Code)
}

func TestAppDelete(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	fs.DomainsStore.ListByAppFn = func(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
		return nil, nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.DELETE("/apps/:id", h.Delete)
	assert.Equal(t, 204, doJSON(r, "DELETE", "/apps/"+id.String(), nil).Code)
}

func TestAppScaleValidation(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/scale", h.Scale)
	// missing replicas, but valid id
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/"+uuid.New().String()+"/scale", map[string]any{}).Code)
}

func TestAppUpdateEnvBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.PUT("/apps/:id/env", h.UpdateEnv)
	assert.Equal(t, 400, doJSON(r, "PUT", "/apps/bad/env", map[string]any{"env_vars": map[string]string{}}).Code)
}

func TestAppGetStatus(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/status", h.GetStatus)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps/"+id.String()+"/status", nil).Code)
}

func TestAppGetPods(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/pods", h.GetPods)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps/"+id.String()+"/pods", nil).Code)
}

func TestAppGetMetrics(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/metrics", h.GetMetrics)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps/"+uuid.New().String()+"/metrics", nil).Code)
}

func TestAppRestartBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/restart", h.Restart)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/restart", nil).Code)
}

func TestAppClearBuildCache(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/clear-cache", h.ClearBuildCache)
	assert.Equal(t, 200, doJSON(r, "POST", "/apps/"+id.String()+"/clear-cache", nil).Code)
}

func TestAppStopBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/stop", h.Stop)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/stop", nil).Code)
}

func TestAppUpdateBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.PUT("/apps/:id", h.Update)
	assert.Equal(t, 400, doJSON(r, "PUT", "/apps/bad", map[string]any{}).Code)
}

func TestAppWebhookEndpointsBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/webhook/enable", h.EnableWebhook)
	r.POST("/apps/:id/webhook/disable", h.DisableWebhook)
	r.POST("/apps/:id/webhook/regenerate", h.RegenerateWebhook)
	r.GET("/apps/:id/webhook", h.GetWebhookConfig)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/webhook/enable", nil).Code)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/webhook/disable", nil).Code)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/webhook/regenerate", nil).Code)
	assert.Equal(t, 400, doJSON(r, "GET", "/apps/bad/webhook", nil).Code)
}

func TestAppEnableWebhookSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/webhook/enable", h.EnableWebhook)
	assert.Equal(t, 200, doJSON(r, "POST", "/apps/"+id.String()+"/webhook/enable", nil).Code)
}

func TestAppGetSecrets(t *testing.T) {
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

func TestAppUpdateSecretsBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.PUT("/apps/:id/secrets", h.UpdateSecrets)
	assert.Equal(t, 400, doJSON(r, "PUT", "/apps/bad/secrets", map[string]string{"K": "V"}).Code)
}

func TestAppGetPodEventsBadID(t *testing.T) {
	h := newAppHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/pods/:podName/events", h.GetPodEvents)
	assert.Equal(t, 400, doJSON(r, "GET", "/apps/bad/pods/p1/events", nil).Code)
}

func TestAppGetPodEventsSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Application, error) {
		return appWithID(id), nil
	}
	h := newAppHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/pods/:podName/events", h.GetPodEvents)
	assert.Equal(t, 200, doJSON(r, "GET", "/apps/"+id.String()+"/pods/web-0/events", nil).Code)
}
