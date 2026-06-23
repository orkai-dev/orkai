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

func newDeployHandler(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *DeployHandler {
	build := service.NewBuildService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testLogger())
	settings := service.NewSettingService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger())
	notif := service.NewNotificationService(fs, settings, testLogger())
	ra := service.NewRegistryAuth(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testLogger())
	dep := service.NewDeployService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), build, notif, ra, testsupport.NewFakeEnqueuer())
	return NewDeployHandler(dep, fs, service.NewAuthz(fs))
}

func TestDeployTriggerBadID(t *testing.T) {
	h := newDeployHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/deploy", h.Trigger)
	assert.Equal(t, 400, doJSON(r, "POST", "/apps/bad/deploy", nil).Code)
}

func TestDeployTriggerForbidden(t *testing.T) {
	fs := testsupport.NewFakeStore()
	wireAppOrg(fs, uuid.New())
	h := newDeployHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/apps/:id/deploy", h.Trigger)
	assert.Equal(t, 403, doJSON(r, "POST", "/apps/"+uuid.New().String()+"/deploy", nil).Code)
}

func TestDeployGetBadID(t *testing.T) {
	h := newDeployHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/deployments/:id", h.Get)
	assert.Equal(t, 400, doJSON(r, "GET", "/deployments/bad", nil).Code)
}

func TestDeployListBadID(t *testing.T) {
	h := newDeployHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/apps/:id/deployments", h.List)
	assert.Equal(t, 400, doJSON(r, "GET", "/apps/bad/deployments", nil).Code)
}

func TestDeployListAll(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.DeploymentListFilter) ([]model.Deployment, int, error) {
		return nil, 0, nil
	}
	h := newDeployHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/deployments", h.ListAll)
	assert.Equal(t, 200, doJSON(r, "GET", "/deployments", nil).Code)
}

func TestDeployListQueue(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DeploymentsStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.DeploymentListFilter) ([]model.Deployment, int, error) {
		return nil, 0, nil
	}
	h := newDeployHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/deployments/queue", h.ListQueue)
	assert.Equal(t, 200, doJSON(r, "GET", "/deployments/queue", nil).Code)
}

func TestDeployCancelBadID(t *testing.T) {
	h := newDeployHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/deployments/:id/cancel", h.Cancel)
	assert.Equal(t, 400, doJSON(r, "POST", "/deployments/bad/cancel", nil).Code)
}

func TestDeployRollbackBadID(t *testing.T) {
	h := newDeployHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/deployments/:id/rollback", h.Rollback)
	assert.Equal(t, 400, doJSON(r, "POST", "/deployments/bad/rollback", nil).Code)
}

func TestPtrUUID(t *testing.T) {
	assert.Nil(t, ptrUUID(uuid.Nil))
	id := uuid.New()
	assert.Equal(t, id, *ptrUUID(id))
}
