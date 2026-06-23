package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

func TestVersionGetVersionInfoDefault(t *testing.T) {
	s := &VersionService{logger: testsupport.NewTestLogger()}
	info := s.GetVersionInfo(context.Background())
	assert.Equal(t, info.Current, info.Latest)
}

func TestVersionGetVersionInfoCached(t *testing.T) {
	s := &VersionService{logger: testsupport.NewTestLogger()}
	s.setCached("v1.0.0", "v1.2.0", true)
	info := s.GetVersionInfo(context.Background())
	assert.Equal(t, "v1.0.0", info.Current)
	assert.Equal(t, "v1.2.0", info.Latest)
	assert.True(t, info.UpdateAvail)
}

func TestShouldUpdate(t *testing.T) {
	assert.False(t, shouldUpdate("v1.0.0", "v1.0.0"))
	assert.True(t, shouldUpdate("v1.1.0", "v1.0.0"))
	assert.False(t, shouldUpdate("v1.0.0", "v1.1.0"))
	// pre-release current upgrading to matching stable release
	assert.True(t, shouldUpdate("v1.0.0", "v1.0.0-rc1"))
	// pre-release to a different pre-release of same base: not a stable upgrade
	assert.False(t, shouldUpdate("v1.0.0-rc2", "v1.0.0-rc1"))
}

func TestIsNewer(t *testing.T) {
	assert.True(t, isNewer("v2.0.0", "v1.9.9"))
	assert.True(t, isNewer("v1.3.0", "v1.2.9"))
	assert.True(t, isNewer("v1.2.4", "v1.2.3"))
	assert.False(t, isNewer("v1.2.3", "v1.2.3"))
	assert.False(t, isNewer("v1.0.0", "v2.0.0"))
	assert.True(t, isNewer("v1.2.3-beta", "v1.2.2"))
}

func TestGetUpgradeStatusIdle(t *testing.T) {
	s := &VersionService{logger: testsupport.NewTestLogger()}
	// The status file does not exist in the test environment.
	st := s.GetUpgradeStatus()
	assert.Equal(t, "idle", st.Status)
}

func TestVersionGetVersionInfoUsesTTLCache(t *testing.T) {
	fs := testsupport.NewFakeStore()
	dbCalls := 0
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		dbCalls++
		return `{"current":"v1.0.0","latest":"v1.2.0","update_available":true}`, nil
	}
	s := NewVersionService(fs, testsupport.NewTestLogger())

	info1 := s.GetVersionInfo(context.Background())
	info2 := s.GetVersionInfo(context.Background())

	assert.Equal(t, "v1.2.0", info1.Latest)
	assert.Equal(t, "v1.2.0", info2.Latest)
	assert.Equal(t, 1, dbCalls)
}

func TestVersionGetVersionInfoDedupesConcurrentCacheMiss(t *testing.T) {
	fs := testsupport.NewFakeStore()
	var dbCalls atomic.Int32
	start := make(chan struct{})
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		<-start
		dbCalls.Add(1)
		return `{"current":"v1.0.0","latest":"v1.2.0","update_available":true}`, nil
	}
	s := NewVersionService(fs, testsupport.NewTestLogger())

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			info := s.GetVersionInfo(context.Background())
			assert.Equal(t, "v1.2.0", info.Latest)
		}()
	}
	close(start)
	wg.Wait()
	assert.Equal(t, int32(1), dbCalls.Load())
}

func TestVersionSaveVersionInfoServesWithoutDBRead(t *testing.T) {
	fs := testsupport.NewFakeStore()
	dbCalls := 0
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		dbCalls++
		return "", nil
	}
	s := NewVersionService(fs, testsupport.NewTestLogger())
	s.saveVersionInfo(context.Background(), &VersionInfo{
		Current:     "v1.0.0",
		Latest:      "v1.2.0",
		UpdateAvail: true,
	})

	info := s.GetVersionInfo(context.Background())
	assert.Equal(t, "v1.2.0", info.Latest)
	assert.Equal(t, 0, dbCalls)
}

func TestVersionSetCachedDoesNotPersist(t *testing.T) {
	fs := testsupport.NewFakeStore()
	persisted := false
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		persisted = true
		return nil
	}
	s := NewVersionService(fs, testsupport.NewTestLogger())
	s.setCached("v1.0.0", "", false)
	assert.False(t, persisted)
}

func TestVersionSaveVersionInfoPersists(t *testing.T) {
	fs := testsupport.NewFakeStore()
	var persistedKey string
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		persistedKey = key
		return nil
	}
	s := NewVersionService(fs, testsupport.NewTestLogger())
	s.saveVersionInfo(context.Background(), &VersionInfo{
		Current: "v1.0.0",
		Latest:  "v1.2.0",
	})
	assert.Equal(t, model.SettingVersionInfo, persistedKey)
}

func TestClearUpgradeStatusNoFile(t *testing.T) {
	s := &VersionService{logger: testsupport.NewTestLogger()}
	// Should not panic even when the file is absent.
	s.ClearUpgradeStatus()
}
