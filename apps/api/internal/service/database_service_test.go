package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDatabaseService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *DatabaseService {
	return NewDatabaseService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewTestLogger(), testsupport.NewFakeEnqueuer(), testsupport.NewProviders(fs), nil)
}

func validCreateDBInput() CreateDatabaseInput {
	return CreateDatabaseInput{
		ProjectID: uuid.New(),
		Name:      "mydb",
		Engine:    model.DBPostgres,
		Version:   "18",
	}
}

func TestDatabaseCreateInvalidVersion(t *testing.T) {
	s := newDatabaseService(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	in := validCreateDBInput()
	in.Version = "99"
	_, err := s.Create(context.Background(), in)
	require.ErrorContains(t, err, "unsupported version")
}

func TestDatabaseCreateInvalidName(t *testing.T) {
	s := newDatabaseService(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	in := validCreateDBInput()
	in.Name = "-bad name"
	_, err := s.Create(context.Background(), in)
	require.ErrorContains(t, err, "must start with alphanumeric")
}

func TestDatabaseCreateInvalidDBName(t *testing.T) {
	s := newDatabaseService(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	in := validCreateDBInput()
	in.DatabaseName = "bad name!"
	_, err := s.Create(context.Background(), in)
	require.ErrorContains(t, err, "invalid characters")
}

func TestDatabaseCreateProjectNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return nil, errors.New("missing")
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), validCreateDBInput())
	require.ErrorContains(t, err, "project not found")
}

func TestDatabaseCreateNoNamespace(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), validCreateDBInput())
	require.ErrorContains(t, err, "no namespace")
}

func TestDatabaseCreateDBNameConflict(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	fs.ManagedDatabasesStore.ExistsByK8sNameFn = func(ctx context.Context, pid uuid.UUID, k8sName string) (bool, error) {
		return true, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), validCreateDBInput())
	require.ErrorContains(t, err, "database with K8s name")
}

func TestDatabaseCreateAppNameConflict(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	fs.ApplicationsStore.ExistsByK8sNameFn = func(ctx context.Context, pid uuid.UUID, k8sName string) (bool, error) {
		return true, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), validCreateDBInput())
	require.ErrorContains(t, err, "application with K8s name")
}

func TestDatabaseCreateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	fs.ManagedDatabasesStore.CreateFn = func(ctx context.Context, db *model.ManagedDatabase) error {
		return errors.New("insert failed")
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), validCreateDBInput())
	require.Error(t, err)
}

func TestDatabaseCreateOrchFailRollback(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	deleted := false
	fs.ManagedDatabasesStore.DeleteFn = func(ctx context.Context, id uuid.UUID) error {
		deleted = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeployDatabaseFn = func(ctx context.Context, db *model.ManagedDatabase) error {
		return errors.New("deploy failed")
	}
	s := newDatabaseService(fs, orch)
	_, err := s.Create(context.Background(), validCreateDBInput())
	require.Error(t, err)
	assert.True(t, deleted)
}

func TestDatabaseCreateSuccessDefaults(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{Namespace: "ns"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	db, err := s.Create(context.Background(), validCreateDBInput())
	require.NoError(t, err)
	assert.Equal(t, "1Gi", db.StorageSize)
	assert.Equal(t, "500m", db.CPULimit)
	assert.Equal(t, "512Mi", db.MemLimit)
	assert.Equal(t, "mydb", db.DatabaseName)
}

func TestDatabaseGetByID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: id}}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	got, err := s.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
}

func TestDatabaseListSyncStatus(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.ListByProjectFn = func(ctx context.Context, pid uuid.UUID, p store.ListParams) ([]model.ManagedDatabase, int, error) {
		return []model.ManagedDatabase{
			{Status: model.AppStatusIdle},
			{Status: model.AppStatusDeploying},
		}, 2, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.GetDatabaseStatusFn = func(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.AppStatus, error) {
		return &orchestrator.AppStatus{Phase: "running"}, nil
	}
	s := newDatabaseService(fs, orch)
	dbs, total, err := s.List(context.Background(), uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, dbs, 2)
}

func TestDatabaseDelete(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{ExternalEnabled: true}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Delete(context.Background(), uuid.New()))
}

func TestDatabaseDeleteGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return nil, errors.New("missing")
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.Error(t, s.Delete(context.Background(), uuid.New()))
}

func TestDatabaseDeleteOrchError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.DeleteDatabaseFn = func(ctx context.Context, db *model.ManagedDatabase) error {
		return errors.New("k8s failed")
	}
	s := newDatabaseService(fs, orch)
	require.Error(t, s.Delete(context.Background(), uuid.New()))
}

func TestDatabaseGetCredentials(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	creds, err := s.GetCredentials(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotEmpty(t, creds.Host)
}

func TestDatabaseGetCredentialsGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return nil, errors.New("missing")
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.GetCredentials(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestDatabaseGetStatusReconciles(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Status: model.AppStatusDeploying}, nil
	}
	updated := false
	fs.ManagedDatabasesStore.UpdateFn = func(ctx context.Context, db *model.ManagedDatabase) error {
		updated = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.GetDatabaseStatusFn = func(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.AppStatus, error) {
		return &orchestrator.AppStatus{Phase: "running"}, nil
	}
	s := newDatabaseService(fs, orch)
	st, err := s.GetStatus(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "running", st.Phase)
	assert.True(t, updated)
}

func TestDatabaseGetPods(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.GetPods(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestDatabaseUpdateExternalAccessEnable(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.EnableExternalAccessFn = func(ctx context.Context, db *model.ManagedDatabase) (int32, error) {
		return 30005, nil
	}
	s := newDatabaseService(fs, orch)
	db, err := s.UpdateExternalAccess(context.Background(), uuid.New(), true, 30005)
	require.NoError(t, err)
	assert.True(t, db.ExternalEnabled)
	assert.Equal(t, int32(30005), db.ExternalPort)
}

func TestDatabaseUpdateExternalAccessBadPort(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.UpdateExternalAccess(context.Background(), uuid.New(), true, 100)
	require.ErrorContains(t, err, "NodePort must be between")
}

func TestDatabaseUpdateExternalAccessPortConflict(t *testing.T) {
	fs := testsupport.NewFakeStore()
	dbID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: dbID}}, nil
	}
	fs.ManagedDatabasesStore.FindByExternalPortFn = func(ctx context.Context, port int32) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "other"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.UpdateExternalAccess(context.Background(), dbID, true, 30005)
	require.ErrorContains(t, err, "already used")
}

func TestDatabaseUpdateExternalAccessDisable(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{ExternalEnabled: true, ExternalPort: 30005}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	db, err := s.UpdateExternalAccess(context.Background(), uuid.New(), false, 0)
	require.NoError(t, err)
	assert.False(t, db.ExternalEnabled)
	assert.Zero(t, db.ExternalPort)
}

func TestDatabaseUpdateBackupConfig(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orgID := uuid.New()
	s3ID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{ProjectID: uuid.New()}, nil
	}
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceObjectStorage, OrgID: orgID, Name: "s3"}, nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{OrgID: orgID}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	db, err := s.UpdateBackupConfig(context.Background(), uuid.New(), UpdateBackupInput{Enabled: true, Schedule: "0 0 * * *", S3ID: &s3ID})
	require.NoError(t, err)
	assert.True(t, db.BackupEnabled)
}

func TestDatabaseUpdateBackupConfigBadSchedule(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.UpdateBackupConfig(context.Background(), uuid.New(), UpdateBackupInput{Enabled: true, Schedule: "0 0"})
	require.ErrorContains(t, err, "invalid cron schedule")
}

func TestDatabaseUpdateBackupConfigWrongResourceType(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s3ID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{}, nil
	}
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceRegistry, Name: "reg"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.UpdateBackupConfig(context.Background(), uuid.New(), UpdateBackupInput{S3ID: &s3ID})
	require.ErrorContains(t, err, "not an object storage")
}

func TestDatabaseUpdateBackupConfigWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s3ID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{}, nil
	}
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceObjectStorage, OrgID: uuid.New()}, nil
	}
	fs.ProjectsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{OrgID: uuid.New()}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.UpdateBackupConfig(context.Background(), uuid.New(), UpdateBackupInput{S3ID: &s3ID})
	require.ErrorContains(t, err, "does not belong")
}

func TestDatabaseUsedExternalPorts(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.ListExternalPortsFn = func(ctx context.Context) ([]model.ExternalPortInfo, error) {
		return []model.ExternalPortInfo{{Port: 30005}}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	ports, err := s.UsedExternalPorts(context.Background())
	require.NoError(t, err)
	assert.Len(t, ports, 1)
}

func TestDBBackupExt(t *testing.T) {
	assert.Equal(t, "sql", dbBackupExt(model.DBPostgres))
	assert.Equal(t, "sql", dbBackupExt(model.DBMySQL))
	assert.Equal(t, "archive", dbBackupExt(model.DBMongo))
	assert.Equal(t, "rdb", dbBackupExt(model.DBRedis))
	assert.Equal(t, "rdb", dbBackupExt(model.DBValkey))
	assert.Equal(t, "dump", dbBackupExt(model.DBEngine("weird")))
}

func TestDBSafeName(t *testing.T) {
	assert.Equal(t, "k8s", dbSafeName(&model.ManagedDatabase{K8sName: "k8s", Name: "n"}))
	assert.Equal(t, "n", dbSafeName(&model.ManagedDatabase{Name: "n"}))
}

func TestLoadBackupStorage(t *testing.T) {
	fs := testsupport.NewFakeStore()
	raw, _ := json.Marshal(orchestrator.S3Config{Bucket: "b", Region: "us-east-1"})
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Provider: "aws_s3", Config: raw}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())

	got, err := s.loadBackupStorage(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, got)

	id := uuid.New()
	got, err = s.loadBackupStorage(context.Background(), &id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws_s3", got.Provider)
}

func TestTriggerBackupEnqueues(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db", Engine: model.DBPostgres}, nil
	}
	var createdID uuid.UUID
	fs.DatabaseBackupsStore.CreateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		b.ID = uuid.New()
		createdID = b.ID
		return nil
	}
	enq := testsupport.NewFakeEnqueuer()
	s := NewDatabaseService(fs, testsupport.NewFakeTargetRegistry(testsupport.NewFakeOrchestrator()), testsupport.NewTestLogger(), enq, testsupport.NewProviders(fs), nil)
	b, err := s.TriggerBackup(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "pending", b.Status)

	job, ok := enq.LastJob()
	require.True(t, ok)
	assert.Equal(t, jobs.JobDatabaseBackup, job.Type)
	require.NotNil(t, job.DatabaseBackupID)
	assert.Equal(t, createdID, *job.DatabaseBackupID)
}

func TestTriggerBackupEnqueueFailureMarksFailed(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db", Engine: model.DBPostgres}, nil
	}
	fs.DatabaseBackupsStore.CreateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		b.ID = uuid.New()
		return nil
	}
	var updated *model.DatabaseBackup
	fs.DatabaseBackupsStore.UpdateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		updated = b
		return nil
	}
	enq := testsupport.NewFakeEnqueuer()
	enq.Err = errors.New("queue down")
	s := NewDatabaseService(fs, testsupport.NewFakeTargetRegistry(testsupport.NewFakeOrchestrator()), testsupport.NewTestLogger(), enq, testsupport.NewProviders(fs), nil)
	_, err := s.TriggerBackup(context.Background(), uuid.New())
	require.ErrorContains(t, err, "enqueue database backup job")
	require.NotNil(t, updated)
	assert.Equal(t, "failed", updated.Status)
}

func TestRunDatabaseBackupJobSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{Status: "pending"}, nil
	}
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db", Engine: model.DBPostgres}, nil
	}
	var updated *model.DatabaseBackup
	fs.DatabaseBackupsStore.UpdateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		updated = b
		return nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.RunDatabaseBackupJob(context.Background(), uuid.New()))
	require.NotNil(t, updated)
	assert.Equal(t, "running", updated.Status)
	assert.NotEmpty(t, updated.FilePath)
}

func TestRunDatabaseBackupJobSkipsTerminal(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{Status: "completed"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.RunDatabaseBackupJob(context.Background(), uuid.New()))
}

func TestRunDatabaseBackupJobNotDeployed(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{Status: "pending"}, nil
	}
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db"}, nil
	}
	var updated *model.DatabaseBackup
	fs.DatabaseBackupsStore.UpdateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		updated = b
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.GetDatabaseStatusFn = func(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.AppStatus, error) {
		return &orchestrator.AppStatus{Phase: "not deployed"}, nil
	}
	s := newDatabaseService(fs, orch)
	// Terminal business failure: recorded on the row, no error returned (no redelivery).
	require.NoError(t, s.RunDatabaseBackupJob(context.Background(), uuid.New()))
	require.NotNil(t, updated)
	assert.Equal(t, "failed", updated.Status)
}

func TestRunDatabaseBackupJobRedeliveryReconcilesCompleted(t *testing.T) {
	fs := testsupport.NewFakeStore()
	// Redelivery: record already "running" with a FilePath baked in.
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{Status: "running", FilePath: "orkai/db-backups/existing.sql"}, nil
	}
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db", Engine: model.DBPostgres}, nil
	}
	var lastStatus string
	fs.DatabaseBackupsStore.UpdateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		lastStatus = b.Status
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.GetBackupJobStatusFn = func(ctx context.Context, backupID uuid.UUID) string { return "completed" }
	relaunched := false
	orch.RunDatabaseBackupFn = func(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error {
		relaunched = true
		return nil
	}
	s := newDatabaseService(fs, orch)
	require.NoError(t, s.RunDatabaseBackupJob(context.Background(), uuid.New()))
	assert.Equal(t, "completed", lastStatus)
	assert.False(t, relaunched, "must not relaunch a backup whose K8s job already completed")
}

func TestRunDatabaseBackupJobRedeliveryReusesS3Key(t *testing.T) {
	fs := testsupport.NewFakeStore()
	const existingKey = "orkai/db-backups/postgres-db/20260101-000000-abcd1234.sql"
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{Status: "running", FilePath: existingKey}, nil
	}
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db", Engine: model.DBPostgres}, nil
	}
	fs.DatabaseBackupsStore.UpdateFn = func(ctx context.Context, b *model.DatabaseBackup) error { return nil }
	orch := testsupport.NewFakeOrchestrator()
	// No terminal job yet (e.g. crash before the K8s job was created): fall through
	// to an idempotent relaunch that must reuse the original S3 key.
	orch.GetBackupJobStatusFn = func(ctx context.Context, backupID uuid.UUID) string { return "" }
	var savedPath string
	fs.DatabaseBackupsStore.UpdateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		savedPath = b.FilePath
		return nil
	}
	orch.RunDatabaseBackupFn = func(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error {
		return nil
	}
	s := newDatabaseService(fs, orch)
	require.NoError(t, s.RunDatabaseBackupJob(context.Background(), uuid.New()))
	assert.Equal(t, existingKey, savedPath, "redelivery must reuse the original S3 key, not regenerate it")
}

func TestRunDatabaseBackupJobRunError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{Status: "pending"}, nil
	}
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db", Engine: model.DBPostgres}, nil
	}
	var lastStatus string
	fs.DatabaseBackupsStore.UpdateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		lastStatus = b.Status
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.RunDatabaseBackupFn = func(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error {
		return errors.New("backup failed")
	}
	s := newDatabaseService(fs, orch)
	require.NoError(t, s.RunDatabaseBackupJob(context.Background(), uuid.New()))
	assert.Equal(t, "failed", lastStatus)
}

func TestRunDueBackupsTriggersMatchingSchedule(t *testing.T) {
	due := &model.ManagedDatabase{Name: "due", Engine: model.DBPostgres, BackupEnabled: true, BackupSchedule: "* * * * *"}
	notDue := &model.ManagedDatabase{Name: "not-due", Engine: model.DBPostgres, BackupEnabled: true, BackupSchedule: "0 0 31 2 *"}

	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.ListBackupEnabledFn = func(ctx context.Context) ([]model.ManagedDatabase, error) {
		return []model.ManagedDatabase{*due, *notDue}, nil
	}
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return due, nil
	}

	var created int
	fs.DatabaseBackupsStore.CreateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		created++
		b.ID = uuid.New()
		return nil
	}

	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	s.runDueBackups(context.Background())

	// "0 0 31 2 *" (Feb 31) can never match, so only the wildcard schedule fires.
	assert.Equal(t, 1, created)
}

func TestRunDueBackupsContinuesAfterError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.ListBackupEnabledFn = func(ctx context.Context) ([]model.ManagedDatabase, error) {
		return []model.ManagedDatabase{
			{Name: "a", Engine: model.DBPostgres, BackupEnabled: true, BackupSchedule: "* * * * *"},
			{Name: "b", Engine: model.DBPostgres, BackupEnabled: true, BackupSchedule: "* * * * *"},
		}, nil
	}
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "x", Engine: model.DBPostgres}, nil
	}

	var created int
	fs.DatabaseBackupsStore.CreateFn = func(ctx context.Context, b *model.DatabaseBackup) error {
		created++
		b.ID = uuid.New()
		return nil
	}

	orch := testsupport.NewFakeOrchestrator()
	orch.RunDatabaseBackupFn = func(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error {
		return errors.New("backup failed")
	}

	s := newDatabaseService(fs, orch)
	s.runDueBackups(context.Background())

	// Both databases are attempted even though every backup run fails.
	assert.Equal(t, 2, created)
}

func TestListBackupsReconciles(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DatabaseBackupsStore.ListByDatabaseFn = func(ctx context.Context, id uuid.UUID, p store.ListParams) ([]model.DatabaseBackup, int, error) {
		return []model.DatabaseBackup{
			{Status: "running"},
			{RestoreStatus: "running"},
		}, 2, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	backups, total, err := s.ListBackups(context.Background(), uuid.New(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, "completed", backups[0].Status)
	assert.Equal(t, "completed", backups[1].RestoreStatus)
}

func TestListBackupsStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.DatabaseBackupsStore.ListByDatabaseFn = func(ctx context.Context, id uuid.UUID, p store.ListParams) ([]model.DatabaseBackup, int, error) {
		return nil, 0, errors.New("query failed")
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	_, _, err := s.ListBackups(context.Background(), uuid.New(), store.DefaultListParams())
	require.Error(t, err)
}

func TestRestoreBackupSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	dbID := uuid.New()
	cfg := orchestrator.S3Config{Bucket: "b"}
	raw, _ := json.Marshal(cfg)
	s3ID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: dbID}, Name: "db", BackupS3ID: &s3ID, Engine: model.DBPostgres}, nil
	}
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{DatabaseID: dbID, Status: "completed", FilePath: "key.sql"}, nil
	}
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Config: raw}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.RestoreBackup(context.Background(), dbID, uuid.New()))
}

func TestRestoreBackupNotRunning(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db"}, nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.GetDatabaseStatusFn = func(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.AppStatus, error) {
		return &orchestrator.AppStatus{Phase: "stopped"}, nil
	}
	s := newDatabaseService(fs, orch)
	require.ErrorContains(t, s.RestoreBackup(context.Background(), uuid.New(), uuid.New()), "must be running")
}

func TestRestoreBackupWrongDatabase(t *testing.T) {
	fs := testsupport.NewFakeStore()
	dbID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: dbID}, Name: "db"}, nil
	}
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{DatabaseID: uuid.New(), Status: "completed"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.ErrorContains(t, s.RestoreBackup(context.Background(), dbID, uuid.New()), "does not belong")
}

func TestRestoreBackupNotCompleted(t *testing.T) {
	fs := testsupport.NewFakeStore()
	dbID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: dbID}, Name: "db"}, nil
	}
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{DatabaseID: dbID, Status: "running"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.ErrorContains(t, s.RestoreBackup(context.Background(), dbID, uuid.New()), "completed backup")
}

func TestRestoreBackupNoS3(t *testing.T) {
	fs := testsupport.NewFakeStore()
	dbID := uuid.New()
	fs.ManagedDatabasesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{BaseModel: model.BaseModel{ID: dbID}, Name: "db"}, nil
	}
	fs.DatabaseBackupsStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
		return &model.DatabaseBackup{DatabaseID: dbID, Status: "completed"}, nil
	}
	s := newDatabaseService(fs, testsupport.NewFakeOrchestrator())
	require.ErrorContains(t, s.RestoreBackup(context.Background(), dbID, uuid.New()), "S3 storage is not configured")
}
