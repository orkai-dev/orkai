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

func newDatabaseHandler(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *DatabaseHandler {
	return NewDatabaseHandler(service.NewDatabaseService(fs, testsupport.NewFakeTargetRegistry(orch), testLogger(), testsupport.NewFakeEnqueuer(), testsupport.NewProviders(fs), nil), fs, service.NewAuthz(fs))
}

func dbWithID(id uuid.UUID) *model.ManagedDatabase {
	return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: id}, Name: "db", Engine: model.DBPostgres, Status: model.AppStatusRunning}
}

func TestDatabaseListByProjectBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id/databases", h.ListByProject)
	assert.Equal(t, 400, doJSON(r, "GET", "/projects/bad/databases", nil).Code)
}

func TestDatabaseListAll(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.ListAllFn = func(ctx context.Context, p store.ListParams, f store.DatabaseListFilter) ([]model.ManagedDatabase, int, error) {
		return []model.ManagedDatabase{{}}, 1, nil
	}
	h := newDatabaseHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases", h.ListAll)
	assert.Equal(t, 200, doJSON(r, "GET", "/databases", nil).Code)
}

func TestDatabaseListByProject(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.ListByProjectFn = func(ctx context.Context, pid uuid.UUID, p store.ListParams) ([]model.ManagedDatabase, int, error) {
		return nil, 0, nil
	}
	h := newDatabaseHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/projects/:id/databases", h.ListByProject)
	assert.Equal(t, 200, doJSON(r, "GET", "/projects/"+uuid.New().String()+"/databases", nil).Code)
}

func TestDatabaseCreateValidation(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/databases", h.Create)
	assert.Equal(t, 400, doJSON(r, "POST", "/databases", map[string]any{}).Code)
}

func TestDatabaseGetBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/:id", h.Get)
	assert.Equal(t, 400, doJSON(r, "GET", "/databases/bad", nil).Code)
}

func TestDatabaseGetSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.ManagedDatabase, error) {
		return dbWithID(id), nil
	}
	h := newDatabaseHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/:id", h.Get)
	assert.Equal(t, 200, doJSON(r, "GET", "/databases/"+id.String(), nil).Code)
}

func TestDatabaseDeleteBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.DELETE("/databases/:id", h.Delete)
	assert.Equal(t, 400, doJSON(r, "DELETE", "/databases/bad", nil).Code)
}

func TestDatabaseListVersions(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/versions", h.ListVersions)
	assert.Equal(t, 200, doJSON(r, "GET", "/databases/versions", nil).Code)
	assert.Equal(t, 200, doJSON(r, "GET", "/databases/versions?engine=postgres", nil).Code)
	assert.Equal(t, 400, doJSON(r, "GET", "/databases/versions?engine=nope", nil).Code)
}

func TestDatabaseGetCredentialsBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/:id/credentials", h.GetCredentials)
	assert.Equal(t, 400, doJSON(r, "GET", "/databases/bad/credentials", nil).Code)
}

func TestDatabaseGetStatusBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/:id/status", h.GetStatus)
	assert.Equal(t, 400, doJSON(r, "GET", "/databases/bad/status", nil).Code)
}

func TestDatabaseGetPodsBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/:id/pods", h.GetPods)
	assert.Equal(t, 400, doJSON(r, "GET", "/databases/bad/pods", nil).Code)
}

func TestDatabaseTriggerBackupBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/databases/:id/backups", h.TriggerBackup)
	assert.Equal(t, 400, doJSON(r, "POST", "/databases/bad/backups", nil).Code)
}

func TestDatabaseUpdateExternalAccessBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.PUT("/databases/:id/external", h.UpdateExternalAccess)
	assert.Equal(t, 400, doJSON(r, "PUT", "/databases/bad/external", map[string]any{"enabled": true}).Code)
}

func TestDatabaseUsedPorts(t *testing.T) {
	fs := testsupport.NewFakeStore()
	h := newDatabaseHandler(fs, testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/used-ports", h.UsedPorts)
	assert.Equal(t, 200, doJSON(r, "GET", "/databases/used-ports", nil).Code)
}

func TestDatabaseUpdateBackupConfigBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.PUT("/databases/:id/backup-config", h.UpdateBackupConfig)
	assert.Equal(t, 400, doJSON(r, "PUT", "/databases/bad/backup-config", map[string]any{}).Code)
}

func TestDatabaseRestoreBackupBadIDs(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.POST("/databases/:id/backups/:backupId/restore", h.RestoreBackup)
	assert.Equal(t, 400, doJSON(r, "POST", "/databases/bad/backups/"+uuid.New().String()+"/restore", nil).Code)
	assert.Equal(t, 400, doJSON(r, "POST", "/databases/"+uuid.New().String()+"/backups/bad/restore", nil).Code)
}

func TestDatabaseListBackupsBadID(t *testing.T) {
	h := newDatabaseHandler(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	r, _, _ := newAuthedRouter()
	r.GET("/databases/:id/backups", h.ListBackups)
	assert.Equal(t, 400, doJSON(r, "GET", "/databases/bad/backups", nil).Code)
}
