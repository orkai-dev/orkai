package v1

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ProjectHandler ──────────────────────────────────────────────

func TestProjectHandlerList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, p store.ListParams) ([]model.Project, int, error) {
		return []model.Project{{}}, 1, nil
	}
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.GET("/projects", h.List)
	w := doJSON(r, "GET", "/projects", nil)
	assert.Equal(t, 200, w.Code)
}

func TestProjectHandlerCreateValidationError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.POST("/projects", h.Create)
	w := doJSON(r, "POST", "/projects", map[string]any{}) // missing required name
	assert.Equal(t, 400, w.Code)
}

func TestProjectHandlerGetBadID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id", h.Get)
	w := doJSON(r, "GET", "/projects/not-a-uuid", nil)
	assert.Equal(t, 400, w.Code)
}

func TestProjectHandlerGetNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, errors.New("missing")
	}
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id", h.Get)
	w := doJSON(r, "GET", "/projects/"+uuid.New().String(), nil)
	assert.Equal(t, 404, w.Code)
}

func TestProjectHandlerGetSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	pid := uuid.New()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: pid}}, nil
	}
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id", h.Get)
	w := doJSON(r, "GET", "/projects/"+pid.String(), nil)
	assert.Equal(t, 200, w.Code)
}

func TestProjectHandlerDelete(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.DELETE("/projects/:id", h.Delete)
	w := doJSON(r, "DELETE", "/projects/"+uuid.New().String(), nil)
	assert.Equal(t, 204, w.Code)
}

func TestProjectHandlerDeleteBadID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.DELETE("/projects/:id", h.Delete)
	w := doJSON(r, "DELETE", "/projects/bad", nil)
	assert.Equal(t, 400, w.Code)
}

func TestProjectHandlerUpdateBadID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.PUT("/projects/:id", h.Update)
	w := doJSON(r, "PUT", "/projects/bad", map[string]any{})
	assert.Equal(t, 400, w.Code)
}

func TestProjectHandlerUpdateEnvBadID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := NewProjectHandler(service.NewProjectService(fs, testsupport.NewFakeTargetRegistry(), testLogger(), nil))
	r, _, _ := newAuthedRouter()
	r.PUT("/projects/:id/env", h.UpdateEnv)
	w := doJSON(r, "PUT", "/projects/bad/env", map[string]any{"env_vars": map[string]string{}})
	assert.Equal(t, 400, w.Code)
}

// ─── TemplateHandler ─────────────────────────────────────────────

func TestTemplateHandlerList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.TemplatesStore.ListFn = func(ctx context.Context, p store.ListParams) ([]model.Template, int, error) {
		return []model.Template{{}}, 1, nil
	}
	h := NewTemplateHandler(service.NewTemplateService(fs, testLogger()))
	r, _, _ := newAuthedRouter()
	r.GET("/templates", h.List)
	assert.Equal(t, 200, doJSON(r, "GET", "/templates", nil).Code)
}

func TestTemplateHandlerGetBadID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := NewTemplateHandler(service.NewTemplateService(fs, testLogger()))
	r, _, _ := newAuthedRouter()
	r.GET("/templates/:id", h.Get)
	assert.Equal(t, 400, doJSON(r, "GET", "/templates/bad", nil).Code)
}

func TestTemplateHandlerGetSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.TemplatesStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.Template, error) {
		return &model.Template{BaseModel: model.BaseModel{ID: id}}, nil
	}
	h := NewTemplateHandler(service.NewTemplateService(fs, testLogger()))
	r, _, _ := newAuthedRouter()
	r.GET("/templates/:id", h.Get)
	assert.Equal(t, 200, doJSON(r, "GET", "/templates/"+id.String(), nil).Code)
}

// ─── ResourceHandler ─────────────────────────────────────────────

func newResourceHandler(fs *testsupport.FakeStore) *ResourceHandler {
	return NewResourceHandler(service.NewResourceService(fs, testsupport.NewProviders(fs), testLogger(), nil))
}

func TestResourceHandlerList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, rt string) ([]model.SharedResource, error) {
		return []model.SharedResource{{}}, nil
	}
	r, _, _ := newAuthedRouter()
	r.GET("/resources", newResourceHandler(fs).List)
	assert.Equal(t, 200, doJSON(r, "GET", "/resources", nil).Code)
}

func TestResourceHandlerCreateValidation(t *testing.T) {
	r, _, _ := newAuthedRouter()
	r.POST("/resources", newResourceHandler(testsupport.NewFakeStore()).Create)
	assert.Equal(t, 400, doJSON(r, "POST", "/resources", map[string]any{}).Code)
}

func TestResourceHandlerDeleteBadID(t *testing.T) {
	r, _, _ := newAuthedRouter()
	r.DELETE("/resources/:id", newResourceHandler(testsupport.NewFakeStore()).Delete)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/resources/bad", nil).Code)
}

func TestResourceHandlerUpdateBadID(t *testing.T) {
	r, _, _ := newAuthedRouter()
	r.PUT("/resources/:id", newResourceHandler(testsupport.NewFakeStore()).Update)
	assert.Equal(t, 400, doJSON(r, "PUT", "/resources/bad", map[string]any{}).Code)
}

func TestResourceHandlerTestConnectionBadID(t *testing.T) {
	r, _, _ := newAuthedRouter()
	r.POST("/resources/:id/test", newResourceHandler(testsupport.NewFakeStore()).TestConnection)
	assert.Equal(t, 400, doJSON(r, "POST", "/resources/bad/test", nil).Code)
}

func TestResourceHandlerListReposBadID(t *testing.T) {
	r, _, _ := newAuthedRouter()
	r.GET("/resources/:id/repos", newResourceHandler(testsupport.NewFakeStore()).ListRepos)
	assert.Equal(t, 400, doJSON(r, "GET", "/resources/bad/repos", nil).Code)
}

func TestResourceHandlerGenerateSSHKey(t *testing.T) {
	fs := testsupport.NewFakeStore()
	r, _, _ := newAuthedRouter()
	r.POST("/resources/ssh-key", newResourceHandler(fs).GenerateSSHKey)
	w := doJSON(r, "POST", "/resources/ssh-key", map[string]any{"name": "k", "algorithm": "ed25519"})
	assert.Equal(t, 201, w.Code)
}

// ─── MonitoringHandler ───────────────────────────────────────────

func newMonitoringHandler(ms *testsupport.FakeMetricsStore) *MonitoringHandler {
	mc := service.NewMetricsCollector(ms, testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry(), testLogger(), nil)
	return NewMonitoringHandler(mc)
}

func TestMonitoringSnapshots(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	ms.SnapshotsStore.QueryFn = func(ctx context.Context, q store.SnapshotQuery) ([]model.MetricSnapshot, error) {
		return []model.MetricSnapshot{{}}, nil
	}
	r, _, _ := newAuthedRouter()
	r.GET("/snapshots", newMonitoringHandler(ms).GetSnapshots)
	assert.Equal(t, 200, doJSON(r, "GET", "/snapshots?from=2026-01-01T00:00:00Z&to=2026-02-01T00:00:00Z", nil).Code)
}

func TestMonitoringEvents(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	r, _, _ := newAuthedRouter()
	r.GET("/events", newMonitoringHandler(ms).GetEvents)
	assert.Equal(t, 200, doJSON(r, "GET", "/events?from=2026-01-01T00:00:00Z", nil).Code)
}

func TestMonitoringAlerts(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	r, _, _ := newAuthedRouter()
	r.GET("/alerts", newMonitoringHandler(ms).GetAlerts)
	assert.Equal(t, 200, doJSON(r, "GET", "/alerts?active=true&severity=critical", nil).Code)
}

func TestMonitoringActiveAlerts(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	r, _, _ := newAuthedRouter()
	r.GET("/alerts/active", newMonitoringHandler(ms).GetActiveAlerts)
	assert.Equal(t, 200, doJSON(r, "GET", "/alerts/active", nil).Code)
}

func TestMonitoringResolveAlertBadID(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	r, _, _ := newAuthedRouter()
	r.POST("/alerts/:id/resolve", newMonitoringHandler(ms).ResolveAlert)
	assert.Equal(t, 400, doJSON(r, "POST", "/alerts/bad/resolve", nil).Code)
}

func TestMonitoringResolveAlertSuccess(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	r, _, _ := newAuthedRouter()
	r.POST("/alerts/:id/resolve", newMonitoringHandler(ms).ResolveAlert)
	assert.Equal(t, 200, doJSON(r, "POST", "/alerts/"+uuid.New().String()+"/resolve", nil).Code)
}

// ─── SettingHandler ──────────────────────────────────────────────

func newSettingHandler(fs *testsupport.FakeStore) *SettingHandler {
	return NewSettingHandler(service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(), testLogger()))
}

func TestSettingGetAll(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetAllFn = func(ctx context.Context) ([]model.Setting, error) {
		return []model.Setting{{Key: "k", Value: "v"}}, nil
	}
	r, _, _ := newAuthedRouter()
	r.GET("/settings", newSettingHandler(fs).GetAll)
	assert.Equal(t, 200, doJSON(r, "GET", "/settings", nil).Code)
}

func TestSettingGetAllMasksSecrets(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetAllFn = func(ctx context.Context) ([]model.Setting, error) {
		return []model.Setting{
			{Key: "base_domain", Value: "example.com"},
			{Key: "smtp_password", Value: "hunter2"},
			{Key: "google_oauth_client_secret", Value: ""},
		}, nil
	}
	r, _, _ := newAuthedRouter()
	r.GET("/settings", newSettingHandler(fs).GetAll)
	w := doJSON(r, "GET", "/settings", nil)
	assert.Equal(t, 200, w.Code)

	var out map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "example.com", out["base_domain"])
	assert.Equal(t, model.SettingSecretMask, out["smtp_password"])
	// Empty secret stays empty (not masked).
	assert.Equal(t, "", out["google_oauth_client_secret"])
}

func TestSettingUpdateValidation(t *testing.T) {
	r, _, _ := newAuthedRouter()
	r.PUT("/settings", newSettingHandler(testsupport.NewFakeStore()).Update)
	assert.Equal(t, 400, doJSON(r, "PUT", "/settings", map[string]any{}).Code)
}

func TestSettingUpdateSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	r, _, _ := newAuthedRouter()
	r.PUT("/settings", newSettingHandler(fs).Update)
	assert.Equal(t, 200, doJSON(r, "PUT", "/settings", map[string]any{"key": "panel_domain", "value": ""}).Code)
}

func TestSettingVerifyDomainMissing(t *testing.T) {
	r, _, _ := newAuthedRouter()
	r.GET("/verify-domain", newSettingHandler(testsupport.NewFakeStore()).VerifyDomain)
	assert.Equal(t, 400, doJSON(r, "GET", "/verify-domain", nil).Code)
}
