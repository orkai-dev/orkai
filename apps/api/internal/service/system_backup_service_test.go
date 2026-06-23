package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSystemBackupService builds the service without the scheduler goroutine.
func newSystemBackupService(fs *testsupport.FakeStore) *SystemBackupService {
	return &SystemBackupService{
		store:     fs,
		settings:  newSettingService(fs, testsupport.NewFakeTargetRegistry()),
		dbURL:     "postgres://user:pass@localhost:5432/orkai",
		logger:    testsupport.NewTestLogger(),
		queue:     testsupport.NewFakeEnqueuer(),
		providers: testsupport.NewProviders(fs),
	}
}

func TestSystemBackupGetConfigDefaults(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newSystemBackupService(fs)
	cfg, err := s.GetConfig(context.Background())
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 30, cfg.Retention)
}

func TestSystemBackupGetConfigCustomRetention(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == "system_backup_retention" {
			return "7", nil
		}
		return "", nil
	}
	s := newSystemBackupService(fs)
	cfg, err := s.GetConfig(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 7, cfg.Retention)
}

func TestSystemBackupSaveConfigEnabledRequiresS3(t *testing.T) {
	s := newSystemBackupService(testsupport.NewFakeStore())
	err := s.SaveConfig(context.Background(), &SystemBackupConfig{Enabled: true})
	require.ErrorContains(t, err, "S3 storage resource is required")
}

func TestSystemBackupSaveConfigEnabledRequiresSchedule(t *testing.T) {
	s := newSystemBackupService(testsupport.NewFakeStore())
	err := s.SaveConfig(context.Background(), &SystemBackupConfig{Enabled: true, S3ID: uuid.New().String()})
	require.ErrorContains(t, err, "schedule is required")
}

func TestSystemBackupSaveConfigBadSchedule(t *testing.T) {
	s := newSystemBackupService(testsupport.NewFakeStore())
	err := s.SaveConfig(context.Background(), &SystemBackupConfig{Enabled: true, S3ID: uuid.New().String(), Schedule: "0 0"})
	require.ErrorContains(t, err, "invalid cron schedule")
}

func TestSystemBackupSaveConfigBadS3ID(t *testing.T) {
	s := newSystemBackupService(testsupport.NewFakeStore())
	err := s.SaveConfig(context.Background(), &SystemBackupConfig{S3ID: "not-a-uuid"})
	require.ErrorContains(t, err, "invalid S3 resource ID")
}

func TestSystemBackupSaveConfigWrongResourceType(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceRegistry, Name: "reg"}, nil
	}
	s := newSystemBackupService(fs)
	err := s.SaveConfig(context.Background(), &SystemBackupConfig{S3ID: uuid.New().String()})
	require.ErrorContains(t, err, "not an object storage")
}

func TestSystemBackupSaveConfigSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceObjectStorage}, nil
	}
	s := newSystemBackupService(fs)
	err := s.SaveConfig(context.Background(), &SystemBackupConfig{Enabled: true, S3ID: uuid.New().String(), Schedule: "0 3 * * *", Retention: 5})
	require.NoError(t, err)
}

func TestSystemBackupTriggerNoS3(t *testing.T) {
	s := newSystemBackupService(testsupport.NewFakeStore())
	_, err := s.TriggerBackup(context.Background())
	require.ErrorContains(t, err, "not configured")
}

func TestSystemBackupTriggerBadS3Config(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s3ID := uuid.New()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == "system_backup_s3_id" {
			return s3ID.String(), nil
		}
		return "", nil
	}
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Config: []byte("not json")}, nil
	}
	s := newSystemBackupService(fs)
	_, err := s.TriggerBackup(context.Background())
	require.ErrorContains(t, err, "invalid object storage config")
}

func TestSystemBackupListBackups(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SystemBackupsStore.ListFn = func(ctx context.Context, p store.ListParams) ([]model.SystemBackup, int, error) {
		return []model.SystemBackup{{}}, 1, nil
	}
	s := newSystemBackupService(fs)
	_, total, err := s.ListBackups(context.Background(), store.DefaultListParams())
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}

func TestSystemBackupScanRejectedWithUsers(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 3, nil }
	s := newSystemBackupService(fs)
	_, err := s.ScanS3Backups(context.Background(), orchestrator.S3Config{}, "")
	require.ErrorContains(t, err, "fresh installation")
}

func TestSystemBackupScanCountError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 0, errors.New("db down") }
	s := newSystemBackupService(fs)
	_, err := s.ScanS3Backups(context.Background(), orchestrator.S3Config{}, "")
	require.Error(t, err)
}

func TestSystemBackupRestoreRejectedWithUsers(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.UsersStore.CountFn = func(ctx context.Context) (int, error) { return 1, nil }
	s := newSystemBackupService(fs)
	err := s.RestoreFromS3(context.Background(), orchestrator.S3Config{}, "key")
	require.ErrorContains(t, err, "fresh installation")
}

func TestShouldRunNow(t *testing.T) {
	assert.False(t, shouldRunNow("0 0"))
	assert.True(t, shouldRunNow("* * * * *"))
}

func TestMatchField(t *testing.T) {
	assert.True(t, matchField("*", 5))
	assert.True(t, matchField("*/5", 10))
	assert.False(t, matchField("*/5", 7))
	assert.False(t, matchField("*/0", 7))
	assert.True(t, matchField("7", 7))
	assert.False(t, matchField("7", 8))
	assert.False(t, matchField("notanumber", 8))
	assert.True(t, matchField("0,30", 30))
	assert.False(t, matchField("0,30", 15))
	assert.True(t, matchField("1-5", 3))
	assert.False(t, matchField("1-5", 6))
	assert.True(t, matchField("0-30/15", 30))
	assert.False(t, matchField("0-30/15", 10))
	assert.True(t, matchField("0,15,30,45", 45))
}

func TestParseDatabaseURL(t *testing.T) {
	host, port, user, password, dbname, err := parseDatabaseURL("postgres://u:p@example.com:6543/mydb")
	require.NoError(t, err)
	assert.Equal(t, "example.com", host)
	assert.Equal(t, "6543", port)
	assert.Equal(t, "u", user)
	assert.Equal(t, "p", password)
	assert.Equal(t, "mydb", dbname)

	_, port, _, _, _, err = parseDatabaseURL("postgres://u:p@example.com/mydb")
	require.NoError(t, err)
	assert.Equal(t, "5432", port)
}
