package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNoopForTest() *NoopOrchestrator {
	return NewNoop(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestNoopAppLifecycle(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	app := &model.Application{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "web", DockerImage: "nginx", Replicas: 2}

	require.NoError(t, n.Deploy(ctx, app, DeployOpts{}))
	require.NoError(t, n.Rollback(ctx, app, 0))
	require.NoError(t, n.Scale(ctx, app, 3))
	require.NoError(t, n.UpdateEnvVars(ctx, app, map[string]string{"A": "B"}))
	require.NoError(t, n.Restart(ctx, app))
	require.NoError(t, n.Stop(ctx, app))
	require.NoError(t, n.Delete(ctx, app))
	require.NoError(t, n.DeletePod(ctx, "p1", app))
	require.NoError(t, n.ConfigureHPA(ctx, app, model.AutoscalingConfig{}))
	require.NoError(t, n.DeleteHPA(ctx, app))

	status, err := n.GetStatus(ctx, app)
	require.NoError(t, err)
	assert.Equal(t, "running", status.Phase)

	pods, err := n.GetPods(ctx, app)
	require.NoError(t, err)
	assert.NotEmpty(t, pods)

	events, err := n.GetPodEvents(ctx, app, "web-0")
	require.NoError(t, err)
	assert.NotNil(t, events)

	require.NoError(t, n.EnsureSecret(ctx, app, map[string]string{"x": "y"}))
	require.NoError(t, n.EnsureImagePullSecret(ctx, app, []byte("{}")))
}

func TestNoopDatabase(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	db := &model.ManagedDatabase{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "pg"}

	require.NoError(t, n.DeployDatabase(ctx, db))
	require.NoError(t, n.DeleteDatabase(ctx, db))

	status, err := n.GetDatabaseStatus(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, "running", status.Phase)

	creds, err := n.GetDatabaseCredentials(ctx, db)
	require.NoError(t, err)
	assert.NotEmpty(t, creds.Host)

	pods, err := n.GetDatabasePods(ctx, db)
	require.NoError(t, err)
	assert.NotNil(t, pods)

	bid := uuid.New()
	require.NoError(t, n.RunDatabaseBackup(ctx, db, bid, &ObjectTransfer{}))
	assert.Equal(t, "completed", n.GetBackupJobStatus(ctx, bid))
	require.NoError(t, n.RestoreDatabaseBackup(ctx, db, bid, &ObjectTransfer{}))
	assert.Equal(t, "completed", n.GetRestoreJobStatus(ctx, bid))

	port, err := n.EnableExternalAccess(ctx, db)
	require.NoError(t, err)
	assert.NotZero(t, port)
	require.NoError(t, n.DisableExternalAccess(ctx, db))
}

func TestNoopIngress(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	app := &model.Application{Name: "web"}
	domain := &model.Domain{Host: "example.com"}

	require.NoError(t, n.CreateIngress(ctx, domain, app))
	require.NoError(t, n.UpdateIngress(ctx, domain, app))
	require.NoError(t, n.DeleteIngress(ctx, domain))
	require.NoError(t, n.DeleteIngressByName(ctx, app, "name"))
	assert.NotEmpty(t, n.IngressName(app, "example.com"))
	assert.NotEmpty(t, n.LegacyIngressName(app, "example.com"))

	st, err := n.GetIngressStatus(ctx, domain, app)
	require.NoError(t, err)
	assert.NotNil(t, st)

	_, err = n.GetCertExpiry(ctx, domain, app)
	require.NoError(t, err)

	require.NoError(t, n.EnsurePanelIngress(ctx, "panel.example.com", "a@b.c"))
	require.NoError(t, n.DeletePanelIngress(ctx))
}

func TestNoopStorage(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	name, err := n.CreateVolume(ctx, VolumeOpts{Name: "v", Namespace: "ns", Size: "1Gi"})
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	require.NoError(t, n.DeleteVolume(ctx, "v", "ns"))
	require.NoError(t, n.ExpandVolume(ctx, "v", "ns", "2Gi"))
}

func TestNoopCluster(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()

	nodes, err := n.GetNodes(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, nodes)

	cm, err := n.GetClusterMetrics(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cm)

	_, err = n.GetNamespaceMetrics(ctx, "ns")
	require.NoError(t, err)

	pods, err := n.GetAllPods(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, pods)

	events, err := n.GetClusterEvents(ctx, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	_, err = n.GetPVCs(ctx)
	require.NoError(t, err)

	_, err = n.GetStorageClasses(ctx)
	require.NoError(t, err)

	_, err = n.GetNamespaces(ctx)
	require.NoError(t, err)

	nm, err := n.GetNodeMetrics(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, nm)

	topo, err := n.GetClusterTopology(ctx)
	require.NoError(t, err)
	assert.NotNil(t, topo)

	require.NoError(t, n.SetNodeLabel(ctx, "node", "k", "v"))
	require.NoError(t, n.RemoveNodeLabel(ctx, "node", "k"))

	pools, err := n.GetNodePools(ctx)
	require.NoError(t, err)
	assert.NotNil(t, pools)
}

func TestNoopNamespacesAndConfig(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()

	require.NoError(t, n.CreateNamespace(ctx, "ns"))
	require.NoError(t, n.DeleteNamespace(ctx, "ns"))
	require.NoError(t, n.EnsureConfigMap(ctx, "ns", "cm", map[string]string{"a": "b"}))
	require.NoError(t, n.DeleteConfigMap(ctx, "ns", "cm"))
	require.NoError(t, n.EnsureResourceQuota(ctx, "ns", model.ResourceQuotaConfig{}))
	require.NoError(t, n.DeleteResourceQuota(ctx, "ns"))
	require.NoError(t, n.EnsureNetworkPolicy(ctx, "ns", true))
	require.NoError(t, n.EnsureServiceAccount(ctx, "ns", "sa"))
	require.NoError(t, n.DeleteServiceAccount(ctx, "ns", "sa"))
}

func TestNoopCronJobs(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	cj := &model.CronJob{Name: "job"}

	require.NoError(t, n.CreateCronJob(ctx, cj))
	require.NoError(t, n.UpdateCronJob(ctx, cj))
	require.NoError(t, n.DeleteCronJob(ctx, cj))
	require.NoError(t, n.SuspendCronJob(ctx, cj, true))

	jobName, err := n.TriggerCronJob(ctx, cj)
	require.NoError(t, err)
	assert.NotEmpty(t, jobName)

	_, err = n.GetJobStatus(ctx, cj, jobName)
	require.NoError(t, err)
}

func TestNoopBuild(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	app := &model.Application{Name: "web"}

	var logged string
	res, err := n.Build(ctx, app, BuildOpts{GitRepo: "r", GitBranch: "main", BuildType: "nixpacks", OnLog: func(s string) { logged = s }})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Image)
	assert.NotEmpty(t, logged)

	require.NoError(t, n.EnsureRegistry(ctx))
	require.NoError(t, n.CancelBuild(ctx, app))
	require.NoError(t, n.ClearBuildCache(ctx, app))
}

func TestNoopStreams(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	app := &model.Application{Name: "web", DockerImage: "nginx"}

	rc, err := n.StreamLogs(ctx, app, LogOpts{})
	require.NoError(t, err)
	buf := make([]byte, 16)
	_, _ = rc.Read(buf)
	require.NoError(t, rc.Close())

	rc, err = n.StreamPodLogs(ctx, app, "web-0", LogOpts{})
	require.NoError(t, err)
	require.NoError(t, rc.Close())

	rc, err = n.GetBuildLogs(ctx, "job", "ns")
	require.NoError(t, err)
	data, _ := io.ReadAll(rc)
	assert.NotEmpty(t, data)
	require.NoError(t, rc.Close())
}

func TestNoopExecTerminal(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()
	app := &model.Application{Name: "web"}
	sess, err := n.ExecTerminal(ctx, app, ExecOpts{})
	require.NoError(t, err)
	buf := make([]byte, 8)
	_, _ = sess.Read(buf)
	_, _ = sess.Write([]byte("ls\n"))
	require.NoError(t, sess.Resize(80, 24))
	require.NoError(t, sess.Close())
}

func TestNoopTraefik(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()

	_, err := n.GetTraefikConfig(ctx)
	require.NoError(t, err)
	require.NoError(t, n.UpdateTraefikConfig(ctx, "yaml"))
	require.NoError(t, n.RestartTraefik(ctx))
	st, err := n.GetTraefikStatus(ctx)
	require.NoError(t, err)
	assert.NotNil(t, st)
}

func TestNoopHelmAndDaemonsets(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()

	rel, err := n.GetHelmReleases(ctx)
	require.NoError(t, err)
	assert.NotNil(t, rel)

	ds, err := n.GetDaemonSets(ctx)
	require.NoError(t, err)
	assert.NotNil(t, ds)
}

func TestNoopCleanup(t *testing.T) {
	n := newNoopForTest()
	ctx := context.Background()

	stats, err := n.GetCleanupStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)

	for _, fn := range []func(context.Context) (*CleanupResult, error){
		n.CleanupEvictedPods, n.CleanupFailedPods, n.CleanupCompletedPods,
		n.CleanupStaleReplicaSets, n.CleanupCompletedJobs,
	} {
		res, err := fn(ctx)
		require.NoError(t, err)
		assert.NotNil(t, res)
	}

	hosts := map[string]bool{"example.com": true}
	orphans, err := n.GetOrphanIngresses(ctx, hosts, nil)
	require.NoError(t, err)
	assert.NotNil(t, orphans)

	res, err := n.CleanupOrphanIngresses(ctx, hosts, nil)
	require.NoError(t, err)
	assert.NotNil(t, res)
}
