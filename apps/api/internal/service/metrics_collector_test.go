package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMetricsCollector(ms *testsupport.FakeMetricsStore, fs *testsupport.FakeStore, reg *orchestrator.TargetRegistry) *MetricsCollector {
	return NewMetricsCollector(ms, fs, reg, testsupport.NewTestLogger(), nil)
}

func TestParseMillis(t *testing.T) {
	assert.Equal(t, int64(0), parseMillis(""))
	assert.Equal(t, int64(0), parseMillis("0"))
	assert.Equal(t, int64(250), parseMillis("250m"))
	assert.Equal(t, int64(1000), parseMillis("1"))
	assert.Equal(t, int64(1), parseMillis("1000000n"))
}

func TestParseBytes(t *testing.T) {
	assert.Equal(t, int64(0), parseBytes(""))
	assert.Equal(t, int64(1024), parseBytes("1Ki"))
	assert.Equal(t, int64(1024*1024), parseBytes("1Mi"))
	assert.Equal(t, int64(1024*1024*1024), parseBytes("1Gi"))
	assert.Equal(t, int64(512), parseBytes("512"))
}

func TestRecordAndGetAppMetrics(t *testing.T) {
	mc := newMetricsCollector(testsupport.NewFakeMetricsStore(), testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry())
	appID := uuid.New()
	assert.Empty(t, mc.GetAppMetrics(appID))
	for i := 0; i < appRingMax+10; i++ {
		mc.RecordAppMetric(appID, AppMetricPoint{Time: time.Now(), CPUUsed: float64(i)})
	}
	pts := mc.GetAppMetrics(appID)
	assert.Len(t, pts, appRingMax, "ring buffer should cap at appRingMax")
}

func TestMetricsCollectorQueries(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	ms.SnapshotsStore.QueryFn = func(ctx context.Context, q store.SnapshotQuery) ([]model.MetricSnapshot, error) {
		return []model.MetricSnapshot{{}}, nil
	}
	ms.EventsStore.ListFn = func(ctx context.Context, q store.EventQuery) ([]model.MetricEvent, int, error) {
		return []model.MetricEvent{{}}, 1, nil
	}
	ms.AlertsStore.ListFn = func(ctx context.Context, q store.AlertQuery) ([]model.MetricAlert, int, error) {
		return []model.MetricAlert{{}}, 1, nil
	}
	ms.AlertsStore.ListActiveFn = func(ctx context.Context) ([]model.MetricAlert, error) {
		return []model.MetricAlert{{}}, nil
	}
	mc := newMetricsCollector(ms, testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry())

	snaps, err := mc.GetSnapshots(context.Background(), store.SnapshotQuery{})
	require.NoError(t, err)
	assert.Len(t, snaps, 1)

	_, total, err := mc.GetEvents(context.Background(), store.EventQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	_, total, err = mc.GetAlerts(context.Background(), store.AlertQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	active, err := mc.GetActiveAlerts(context.Background())
	require.NoError(t, err)
	assert.Len(t, active, 1)

	require.NoError(t, mc.ResolveAlert(context.Background(), uuid.New()))
}

func TestMetricsCollectorCollect(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	inserted := false
	ms.SnapshotsStore.InsertBatchFn = func(ctx context.Context, snaps []model.MetricSnapshot) error {
		inserted = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodeMetricsFn = func(ctx context.Context) ([]orchestrator.NodeMetrics, error) {
		return []orchestrator.NodeMetrics{{Name: "n1", CPUUsed: "250m", CPUTotal: "4000m", MemUsed: "1Gi", MemTotal: "8Gi", PodCount: 3}}, nil
	}
	orch.GetAllPodsFn = func(ctx context.Context) ([]orchestrator.PodInfo, error) {
		return []orchestrator.PodInfo{
			{Name: "p1", AppID: "app-1", Resources: orchestrator.ResourceMetrics{CPUUsed: "100m", CPUTotal: "500m", MemUsed: "256Mi", MemTotal: "512Mi"}},
			{Name: "p2", AppID: ""}, // skipped (no app id)
		}, nil
	}
	orch.GetClusterEventsFn = func(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) {
		return []orchestrator.ClusterEvent{
			{Type: "Warning", Reason: "BackOff", Message: "m", Namespace: "default", InvolvedObject: "p1"},
			{Type: "Normal", Reason: "Started"},
		}, nil
	}
	mc := newMetricsCollector(ms, testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry(orch))
	mc.collect(context.Background())
	assert.True(t, inserted)
}

func TestMetricsCollectorCleanup(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	ms.SnapshotsStore.DeleteOlderThanFn = func(ctx context.Context, before time.Time) (int64, error) { return 5, nil }
	ms.EventsStore.DeleteOlderThanFn = func(ctx context.Context, before time.Time) (int64, error) { return 2, nil }
	ms.AlertsStore.DeleteOlderThanFn = func(ctx context.Context, before time.Time) (int64, error) { return 1, nil }
	mc := newMetricsCollector(ms, testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry())
	mc.cleanup(context.Background())
}

func TestMetricsCollectorStartStop(t *testing.T) {
	mc := newMetricsCollector(testsupport.NewFakeMetricsStore(), testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry())
	mc.Start(nil)
	mc.Stop()
}
