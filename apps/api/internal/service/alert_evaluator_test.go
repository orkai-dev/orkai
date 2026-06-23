package service

import (
	"context"
	"database/sql"
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

func newAlertEvaluator(ms *testsupport.FakeMetricsStore, orch *testsupport.FakeOrchestrator) *AlertEvaluator {
	return NewAlertEvaluator(ms, testsupport.NewFakeTargetRegistry(orch), testsupport.NewFakeStore(), testsupport.NewTestLogger(), nil)
}

// ruleByName looks up a builtin rule for direct evaluation.
func ruleByName(ae *AlertEvaluator, name string) AlertRule {
	for _, r := range ae.rules {
		if r.Name == name {
			return r
		}
	}
	panic("rule not found: " + name)
}

func TestAlertRuleNodeCPUHigh(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodeMetricsFn = func(ctx context.Context) ([]orchestrator.NodeMetrics, error) {
		return []orchestrator.NodeMetrics{
			{Name: "hot", CPUUsed: "3800m", CPUTotal: "4000m"},
			{Name: "cool", CPUUsed: "100m", CPUTotal: "4000m"},
		}, nil
	}
	ae := newAlertEvaluator(testsupport.NewFakeMetricsStore(), orch)
	firings := ruleByName(ae, "node_cpu_high").Evaluate(context.Background(), orch)
	require.Len(t, firings, 1)
	assert.Equal(t, "hot", firings[0].SourceName)
}

func TestAlertRuleNodeMemHigh(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodeMetricsFn = func(ctx context.Context) ([]orchestrator.NodeMetrics, error) {
		return []orchestrator.NodeMetrics{{Name: "n", MemUsed: "7600Mi", MemTotal: "8000Mi"}}, nil
	}
	ae := newAlertEvaluator(testsupport.NewFakeMetricsStore(), orch)
	firings := ruleByName(ae, "node_mem_high").Evaluate(context.Background(), orch)
	require.Len(t, firings, 1)
}

func TestAlertRuleNodeNotReady(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) {
		return []orchestrator.NodeInfo{
			{Name: "down", Status: "NotReady"},
			{Name: "up", Status: "Ready"},
		}, nil
	}
	ae := newAlertEvaluator(testsupport.NewFakeMetricsStore(), orch)
	firings := ruleByName(ae, "node_not_ready").Evaluate(context.Background(), orch)
	require.Len(t, firings, 1)
	assert.Equal(t, "down", firings[0].SourceName)
}

func TestAlertRulePodConditions(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	orch.GetAllPodsFn = func(ctx context.Context) ([]orchestrator.PodInfo, error) {
		return []orchestrator.PodInfo{
			{Name: "crash", Containers: []orchestrator.ContainerStatus{{Name: "c", Reason: "CrashLoopBackOff"}}},
			{Name: "oom", Containers: []orchestrator.ContainerStatus{{Name: "c", Reason: "OOMKilled"}}},
			{Name: "pull", Containers: []orchestrator.ContainerStatus{{Name: "c", Reason: "ImagePullBackOff"}}},
			{Name: "sched", Phase: "Pending", Containers: []orchestrator.ContainerStatus{{Name: "c", Reason: "Unschedulable"}}},
		}, nil
	}
	ae := newAlertEvaluator(testsupport.NewFakeMetricsStore(), orch)
	assert.Len(t, ruleByName(ae, "pod_crashloop").Evaluate(context.Background(), orch), 1)
	assert.Len(t, ruleByName(ae, "pod_oom_killed").Evaluate(context.Background(), orch), 1)
	assert.Len(t, ruleByName(ae, "pod_image_pull_error").Evaluate(context.Background(), orch), 1)
	assert.Len(t, ruleByName(ae, "pod_scheduling_failed").Evaluate(context.Background(), orch), 1)
}

func TestAlertRuleK8sWarningEvents(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	orch.GetClusterEventsFn = func(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) {
		return []orchestrator.ClusterEvent{
			{Type: "Warning", Reason: "FailedMount", Message: "stuck", Namespace: "default", InvolvedObject: "p1", LastSeen: time.Now()},
			{Type: "Warning", Reason: "Started", Message: "ignored", LastSeen: time.Now()}, // ignored reason
			{Type: "Warning", Reason: "Evicted", Message: "disk", InvolvedObject: "node1", LastSeen: time.Now()},
			{Type: "Normal", Reason: "Created", LastSeen: time.Now()},
			{Type: "Warning", Reason: "OldStuff", LastSeen: time.Now().Add(-time.Hour)}, // too old
		}, nil
	}
	ae := newAlertEvaluator(testsupport.NewFakeMetricsStore(), orch)
	firings := ruleByName(ae, "k8s_warning_events").Evaluate(context.Background(), orch)
	// One real warning + one aggregated eviction alert.
	assert.Len(t, firings, 2)
}

func TestAlertRulesReturnNilOnError(t *testing.T) {
	orch := testsupport.NewFakeOrchestrator()
	boomNodeMetrics := func(ctx context.Context) ([]orchestrator.NodeMetrics, error) { return nil, assert.AnError }
	orch.GetNodeMetricsFn = boomNodeMetrics
	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) { return nil, assert.AnError }
	orch.GetAllPodsFn = func(ctx context.Context) ([]orchestrator.PodInfo, error) { return nil, assert.AnError }
	orch.GetClusterEventsFn = func(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) { return nil, assert.AnError }
	ae := newAlertEvaluator(testsupport.NewFakeMetricsStore(), orch)
	for _, r := range ae.rules {
		assert.Nil(t, r.Evaluate(context.Background(), orch), r.Name)
	}
}

func TestEvaluateInsertsNewAlert(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	inserted := 0
	ms.AlertsStore.GetActiveByRuleAndSourceFn = func(ctx context.Context, rule, src string) (*model.MetricAlert, error) {
		return nil, sql.ErrNoRows
	}
	ms.AlertsStore.InsertFn = func(ctx context.Context, a *model.MetricAlert) error {
		inserted++
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) {
		return []orchestrator.NodeInfo{{Name: "down", Status: "NotReady"}}, nil
	}
	// Silence other rules so only node_not_ready fires.
	orch.GetNodeMetricsFn = func(ctx context.Context) ([]orchestrator.NodeMetrics, error) { return nil, nil }
	orch.GetAllPodsFn = func(ctx context.Context) ([]orchestrator.PodInfo, error) { return nil, nil }
	orch.GetClusterEventsFn = func(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) { return nil, nil }
	ae := newAlertEvaluator(ms, orch)
	ae.Evaluate(context.Background())
	assert.GreaterOrEqual(t, inserted, 1)
}

func TestEvaluateResolvesClearedAlert(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	ms.AlertsStore.ListActiveFn = func(ctx context.Context) ([]model.MetricAlert, error) {
		return []model.MetricAlert{{RuleName: "node_not_ready", SourceName: "ghost"}}, nil
	}
	resolved := false
	ms.AlertsStore.ResolveFn = func(ctx context.Context, id uuid.UUID) error {
		resolved = true
		return nil
	}
	orch := testsupport.NewFakeOrchestrator()
	// No nodes firing -> the active "ghost" alert should auto-resolve.
	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) {
		return []orchestrator.NodeInfo{{Name: "ok", Status: "Ready"}}, nil
	}
	orch.GetNodeMetricsFn = func(ctx context.Context) ([]orchestrator.NodeMetrics, error) { return nil, nil }
	orch.GetAllPodsFn = func(ctx context.Context) ([]orchestrator.PodInfo, error) { return nil, nil }
	orch.GetClusterEventsFn = func(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) { return nil, nil }
	ae := newAlertEvaluator(ms, orch)
	ae.Evaluate(context.Background())
	assert.True(t, resolved)
}

func TestIsInCooldown(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	recent := time.Now().Add(-5 * time.Minute)
	ms.AlertsStore.ListFn = func(ctx context.Context, q store.AlertQuery) ([]model.MetricAlert, int, error) {
		return []model.MetricAlert{
			{RuleName: "r", SourceName: "s", ResolvedAt: &recent},
		}, 1, nil
	}
	ae := newAlertEvaluator(ms, testsupport.NewFakeOrchestrator())
	assert.True(t, ae.isInCooldown(context.Background(), "r", "s"))
	assert.False(t, ae.isInCooldown(context.Background(), "other", "s"))
}

func TestIsInCooldownListError(t *testing.T) {
	ms := testsupport.NewFakeMetricsStore()
	ms.AlertsStore.ListFn = func(ctx context.Context, q store.AlertQuery) ([]model.MetricAlert, int, error) {
		return nil, 0, assert.AnError
	}
	ae := newAlertEvaluator(ms, testsupport.NewFakeOrchestrator())
	assert.False(t, ae.isInCooldown(context.Background(), "r", "s"))
}
