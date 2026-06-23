package testsupport

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

// FakeOrchestrator embeds NoopOrchestrator (so every method has a working
// default) and exposes function fields that tests can set to override specific
// methods, in particular to exercise error paths.
type FakeOrchestrator struct {
	*orchestrator.NoopOrchestrator

	// AppManager
	DeployFn        func(ctx context.Context, app *model.Application, opts orchestrator.DeployOpts) error
	RollbackFn      func(ctx context.Context, app *model.Application, revision int64) error
	ScaleFn         func(ctx context.Context, app *model.Application, replicas int32) error
	UpdateEnvVarsFn func(ctx context.Context, app *model.Application, envVars map[string]string) error
	RestartFn       func(ctx context.Context, app *model.Application) error
	StopFn          func(ctx context.Context, app *model.Application) error
	DeleteFn        func(ctx context.Context, app *model.Application) error
	GetStatusFn     func(ctx context.Context, app *model.Application) (*orchestrator.AppStatus, error)
	GetPodsFn       func(ctx context.Context, app *model.Application) ([]orchestrator.PodInfo, error)
	ConfigureHPAFn  func(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error
	DeleteHPAFn     func(ctx context.Context, app *model.Application) error

	// NamespaceManager
	CreateNamespaceFn func(ctx context.Context, name string) error
	DeleteNamespaceFn func(ctx context.Context, name string) error

	// ServiceAccountManager
	EnsureServiceAccountFn func(ctx context.Context, namespace, name string) error
	DeleteServiceAccountFn func(ctx context.Context, namespace, name string) error

	// ConfigMapManager
	EnsureConfigMapFn func(ctx context.Context, namespace, name string, data map[string]string) error
	DeleteConfigMapFn func(ctx context.Context, namespace, name string) error

	// ResourceQuotaManager / NetworkPolicyManager
	EnsureResourceQuotaFn func(ctx context.Context, namespace string, quota model.ResourceQuotaConfig) error
	DeleteResourceQuotaFn func(ctx context.Context, namespace string) error
	EnsureNetworkPolicyFn func(ctx context.Context, namespace string, enabled bool) error

	// SecretManager
	EnsureSecretFn          func(ctx context.Context, app *model.Application, secrets map[string]string) error
	EnsureImagePullSecretFn func(ctx context.Context, app *model.Application, dockerConfigJSON []byte) error

	// BuildManager
	BuildFn           func(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error)
	EnsureRegistryFn  func(ctx context.Context) error
	CancelBuildFn     func(ctx context.Context, app *model.Application) error
	ClearBuildCacheFn func(ctx context.Context, app *model.Application) error

	// IngressManager
	CreateIngressFn       func(ctx context.Context, domain *model.Domain, app *model.Application) error
	UpdateIngressFn       func(ctx context.Context, domain *model.Domain, app *model.Application) error
	DeleteIngressFn       func(ctx context.Context, domain *model.Domain) error
	DeleteIngressByNameFn func(ctx context.Context, app *model.Application, name string) error
	GetIngressStatusFn    func(ctx context.Context, domain *model.Domain, app *model.Application) (*orchestrator.IngressStatus, error)
	GetCertExpiryFn       func(ctx context.Context, domain *model.Domain, app *model.Application) (*time.Time, error)
	EnsurePanelIngressFn  func(ctx context.Context, domain, httpsEmail string) error
	DeletePanelIngressFn  func(ctx context.Context) error

	// DatabaseManager
	DeployDatabaseFn         func(ctx context.Context, db *model.ManagedDatabase) error
	DeleteDatabaseFn         func(ctx context.Context, db *model.ManagedDatabase) error
	GetDatabaseStatusFn      func(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.AppStatus, error)
	GetDatabaseCredentialsFn func(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.DatabaseCredentials, error)
	EnableExternalAccessFn   func(ctx context.Context, db *model.ManagedDatabase) (int32, error)
	DisableExternalAccessFn  func(ctx context.Context, db *model.ManagedDatabase) error
	RunDatabaseBackupFn      func(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error
	RestoreDatabaseBackupFn  func(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error
	GetBackupJobStatusFn     func(ctx context.Context, backupID uuid.UUID) string

	// CronJobManager
	CreateCronJobFn  func(ctx context.Context, cj *model.CronJob) error
	UpdateCronJobFn  func(ctx context.Context, cj *model.CronJob) error
	DeleteCronJobFn  func(ctx context.Context, cj *model.CronJob) error
	SuspendCronJobFn func(ctx context.Context, cj *model.CronJob, suspend bool) error
	TriggerCronJobFn func(ctx context.Context, cj *model.CronJob) (string, error)
	GetJobStatusFn   func(ctx context.Context, cj *model.CronJob, jobName string) (string, error)

	// StorageManager
	CreateVolumeFn func(ctx context.Context, opts orchestrator.VolumeOpts) (string, error)
	DeleteVolumeFn func(ctx context.Context, name, namespace string) error
	ExpandVolumeFn func(ctx context.Context, name, namespace, newSize string) error

	// LogStreamer
	StreamLogsFn   func(ctx context.Context, app *model.Application, opts orchestrator.LogOpts) (io.ReadCloser, error)
	ExecTerminalFn func(ctx context.Context, app *model.Application, opts orchestrator.ExecOpts) (orchestrator.TerminalSession, error)

	// ClusterManager
	GetNodesFn          func(ctx context.Context) ([]orchestrator.NodeInfo, error)
	GetClusterMetricsFn func(ctx context.Context) (*orchestrator.ClusterMetrics, error)
	GetNodeMetricsFn    func(ctx context.Context) ([]orchestrator.NodeMetrics, error)
	GetAllPodsFn        func(ctx context.Context) ([]orchestrator.PodInfo, error)
	GetClusterEventsFn  func(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error)
}

// NewFakeOrchestrator returns a FakeOrchestrator backed by a noop default and a
// discard logger.
func NewFakeOrchestrator() *FakeOrchestrator {
	return &FakeOrchestrator{NoopOrchestrator: orchestrator.NewNoop(NewTestLogger())}
}

// ─── AppManager overrides ────────────────────────────────────────

func (f *FakeOrchestrator) Deploy(ctx context.Context, app *model.Application, opts orchestrator.DeployOpts) error {
	if f.DeployFn != nil {
		return f.DeployFn(ctx, app, opts)
	}
	return f.NoopOrchestrator.Deploy(ctx, app, opts)
}

func (f *FakeOrchestrator) Rollback(ctx context.Context, app *model.Application, revision int64) error {
	if f.RollbackFn != nil {
		return f.RollbackFn(ctx, app, revision)
	}
	return f.NoopOrchestrator.Rollback(ctx, app, revision)
}

func (f *FakeOrchestrator) Scale(ctx context.Context, app *model.Application, replicas int32) error {
	if f.ScaleFn != nil {
		return f.ScaleFn(ctx, app, replicas)
	}
	return f.NoopOrchestrator.Scale(ctx, app, replicas)
}

func (f *FakeOrchestrator) UpdateEnvVars(ctx context.Context, app *model.Application, envVars map[string]string) error {
	if f.UpdateEnvVarsFn != nil {
		return f.UpdateEnvVarsFn(ctx, app, envVars)
	}
	return f.NoopOrchestrator.UpdateEnvVars(ctx, app, envVars)
}

func (f *FakeOrchestrator) Restart(ctx context.Context, app *model.Application) error {
	if f.RestartFn != nil {
		return f.RestartFn(ctx, app)
	}
	return f.NoopOrchestrator.Restart(ctx, app)
}

func (f *FakeOrchestrator) Stop(ctx context.Context, app *model.Application) error {
	if f.StopFn != nil {
		return f.StopFn(ctx, app)
	}
	return f.NoopOrchestrator.Stop(ctx, app)
}

func (f *FakeOrchestrator) Delete(ctx context.Context, app *model.Application) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, app)
	}
	return f.NoopOrchestrator.Delete(ctx, app)
}

func (f *FakeOrchestrator) GetStatus(ctx context.Context, app *model.Application) (*orchestrator.AppStatus, error) {
	if f.GetStatusFn != nil {
		return f.GetStatusFn(ctx, app)
	}
	return f.NoopOrchestrator.GetStatus(ctx, app)
}

func (f *FakeOrchestrator) GetPods(ctx context.Context, app *model.Application) ([]orchestrator.PodInfo, error) {
	if f.GetPodsFn != nil {
		return f.GetPodsFn(ctx, app)
	}
	return f.NoopOrchestrator.GetPods(ctx, app)
}

func (f *FakeOrchestrator) ConfigureHPA(ctx context.Context, app *model.Application, cfg model.AutoscalingConfig) error {
	if f.ConfigureHPAFn != nil {
		return f.ConfigureHPAFn(ctx, app, cfg)
	}
	return f.NoopOrchestrator.ConfigureHPA(ctx, app, cfg)
}

func (f *FakeOrchestrator) DeleteHPA(ctx context.Context, app *model.Application) error {
	if f.DeleteHPAFn != nil {
		return f.DeleteHPAFn(ctx, app)
	}
	return f.NoopOrchestrator.DeleteHPA(ctx, app)
}

// ─── NamespaceManager overrides ──────────────────────────────────

func (f *FakeOrchestrator) CreateNamespace(ctx context.Context, name string) error {
	if f.CreateNamespaceFn != nil {
		return f.CreateNamespaceFn(ctx, name)
	}
	return f.NoopOrchestrator.CreateNamespace(ctx, name)
}

func (f *FakeOrchestrator) DeleteNamespace(ctx context.Context, name string) error {
	if f.DeleteNamespaceFn != nil {
		return f.DeleteNamespaceFn(ctx, name)
	}
	return f.NoopOrchestrator.DeleteNamespace(ctx, name)
}

// ─── ServiceAccountManager overrides ─────────────────────────────

func (f *FakeOrchestrator) EnsureServiceAccount(ctx context.Context, namespace, name string) error {
	if f.EnsureServiceAccountFn != nil {
		return f.EnsureServiceAccountFn(ctx, namespace, name)
	}
	return f.NoopOrchestrator.EnsureServiceAccount(ctx, namespace, name)
}

func (f *FakeOrchestrator) DeleteServiceAccount(ctx context.Context, namespace, name string) error {
	if f.DeleteServiceAccountFn != nil {
		return f.DeleteServiceAccountFn(ctx, namespace, name)
	}
	return f.NoopOrchestrator.DeleteServiceAccount(ctx, namespace, name)
}

// ─── ConfigMapManager overrides ──────────────────────────────────

func (f *FakeOrchestrator) EnsureConfigMap(ctx context.Context, namespace, name string, data map[string]string) error {
	if f.EnsureConfigMapFn != nil {
		return f.EnsureConfigMapFn(ctx, namespace, name, data)
	}
	return f.NoopOrchestrator.EnsureConfigMap(ctx, namespace, name, data)
}

func (f *FakeOrchestrator) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	if f.DeleteConfigMapFn != nil {
		return f.DeleteConfigMapFn(ctx, namespace, name)
	}
	return f.NoopOrchestrator.DeleteConfigMap(ctx, namespace, name)
}

// ─── ResourceQuota / NetworkPolicy overrides ─────────────────────

func (f *FakeOrchestrator) EnsureResourceQuota(ctx context.Context, namespace string, quota model.ResourceQuotaConfig) error {
	if f.EnsureResourceQuotaFn != nil {
		return f.EnsureResourceQuotaFn(ctx, namespace, quota)
	}
	return f.NoopOrchestrator.EnsureResourceQuota(ctx, namespace, quota)
}

func (f *FakeOrchestrator) DeleteResourceQuota(ctx context.Context, namespace string) error {
	if f.DeleteResourceQuotaFn != nil {
		return f.DeleteResourceQuotaFn(ctx, namespace)
	}
	return f.NoopOrchestrator.DeleteResourceQuota(ctx, namespace)
}

func (f *FakeOrchestrator) EnsureNetworkPolicy(ctx context.Context, namespace string, enabled bool) error {
	if f.EnsureNetworkPolicyFn != nil {
		return f.EnsureNetworkPolicyFn(ctx, namespace, enabled)
	}
	return f.NoopOrchestrator.EnsureNetworkPolicy(ctx, namespace, enabled)
}

// ─── SecretManager overrides ─────────────────────────────────────

func (f *FakeOrchestrator) EnsureSecret(ctx context.Context, app *model.Application, secrets map[string]string) error {
	if f.EnsureSecretFn != nil {
		return f.EnsureSecretFn(ctx, app, secrets)
	}
	return f.NoopOrchestrator.EnsureSecret(ctx, app, secrets)
}

func (f *FakeOrchestrator) EnsureImagePullSecret(ctx context.Context, app *model.Application, dockerConfigJSON []byte) error {
	if f.EnsureImagePullSecretFn != nil {
		return f.EnsureImagePullSecretFn(ctx, app, dockerConfigJSON)
	}
	return f.NoopOrchestrator.EnsureImagePullSecret(ctx, app, dockerConfigJSON)
}

// ─── BuildManager overrides ──────────────────────────────────────

func (f *FakeOrchestrator) Build(ctx context.Context, app *model.Application, opts orchestrator.BuildOpts) (*orchestrator.BuildResult, error) {
	if f.BuildFn != nil {
		return f.BuildFn(ctx, app, opts)
	}
	return f.NoopOrchestrator.Build(ctx, app, opts)
}

func (f *FakeOrchestrator) EnsureRegistry(ctx context.Context) error {
	if f.EnsureRegistryFn != nil {
		return f.EnsureRegistryFn(ctx)
	}
	return f.NoopOrchestrator.EnsureRegistry(ctx)
}

func (f *FakeOrchestrator) CancelBuild(ctx context.Context, app *model.Application) error {
	if f.CancelBuildFn != nil {
		return f.CancelBuildFn(ctx, app)
	}
	return f.NoopOrchestrator.CancelBuild(ctx, app)
}

func (f *FakeOrchestrator) ClearBuildCache(ctx context.Context, app *model.Application) error {
	if f.ClearBuildCacheFn != nil {
		return f.ClearBuildCacheFn(ctx, app)
	}
	return f.NoopOrchestrator.ClearBuildCache(ctx, app)
}

// ─── IngressManager overrides ────────────────────────────────────

func (f *FakeOrchestrator) CreateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error {
	if f.CreateIngressFn != nil {
		return f.CreateIngressFn(ctx, domain, app)
	}
	return f.NoopOrchestrator.CreateIngress(ctx, domain, app)
}

func (f *FakeOrchestrator) UpdateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error {
	if f.UpdateIngressFn != nil {
		return f.UpdateIngressFn(ctx, domain, app)
	}
	return f.NoopOrchestrator.UpdateIngress(ctx, domain, app)
}

func (f *FakeOrchestrator) DeleteIngress(ctx context.Context, domain *model.Domain) error {
	if f.DeleteIngressFn != nil {
		return f.DeleteIngressFn(ctx, domain)
	}
	return f.NoopOrchestrator.DeleteIngress(ctx, domain)
}

func (f *FakeOrchestrator) DeleteIngressByName(ctx context.Context, app *model.Application, name string) error {
	if f.DeleteIngressByNameFn != nil {
		return f.DeleteIngressByNameFn(ctx, app, name)
	}
	return f.NoopOrchestrator.DeleteIngressByName(ctx, app, name)
}

func (f *FakeOrchestrator) GetIngressStatus(ctx context.Context, domain *model.Domain, app *model.Application) (*orchestrator.IngressStatus, error) {
	if f.GetIngressStatusFn != nil {
		return f.GetIngressStatusFn(ctx, domain, app)
	}
	return f.NoopOrchestrator.GetIngressStatus(ctx, domain, app)
}

func (f *FakeOrchestrator) GetCertExpiry(ctx context.Context, domain *model.Domain, app *model.Application) (*time.Time, error) {
	if f.GetCertExpiryFn != nil {
		return f.GetCertExpiryFn(ctx, domain, app)
	}
	return f.NoopOrchestrator.GetCertExpiry(ctx, domain, app)
}

func (f *FakeOrchestrator) EnsurePanelIngress(ctx context.Context, domain, httpsEmail string) error {
	if f.EnsurePanelIngressFn != nil {
		return f.EnsurePanelIngressFn(ctx, domain, httpsEmail)
	}
	return f.NoopOrchestrator.EnsurePanelIngress(ctx, domain, httpsEmail)
}

func (f *FakeOrchestrator) DeletePanelIngress(ctx context.Context) error {
	if f.DeletePanelIngressFn != nil {
		return f.DeletePanelIngressFn(ctx)
	}
	return f.NoopOrchestrator.DeletePanelIngress(ctx)
}

// ─── DatabaseManager overrides ───────────────────────────────────

func (f *FakeOrchestrator) DeployDatabase(ctx context.Context, db *model.ManagedDatabase) error {
	if f.DeployDatabaseFn != nil {
		return f.DeployDatabaseFn(ctx, db)
	}
	return f.NoopOrchestrator.DeployDatabase(ctx, db)
}

func (f *FakeOrchestrator) DeleteDatabase(ctx context.Context, db *model.ManagedDatabase) error {
	if f.DeleteDatabaseFn != nil {
		return f.DeleteDatabaseFn(ctx, db)
	}
	return f.NoopOrchestrator.DeleteDatabase(ctx, db)
}

func (f *FakeOrchestrator) GetDatabaseStatus(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.AppStatus, error) {
	if f.GetDatabaseStatusFn != nil {
		return f.GetDatabaseStatusFn(ctx, db)
	}
	return f.NoopOrchestrator.GetDatabaseStatus(ctx, db)
}

func (f *FakeOrchestrator) GetDatabaseCredentials(ctx context.Context, db *model.ManagedDatabase) (*orchestrator.DatabaseCredentials, error) {
	if f.GetDatabaseCredentialsFn != nil {
		return f.GetDatabaseCredentialsFn(ctx, db)
	}
	return f.NoopOrchestrator.GetDatabaseCredentials(ctx, db)
}

func (f *FakeOrchestrator) EnableExternalAccess(ctx context.Context, db *model.ManagedDatabase) (int32, error) {
	if f.EnableExternalAccessFn != nil {
		return f.EnableExternalAccessFn(ctx, db)
	}
	return f.NoopOrchestrator.EnableExternalAccess(ctx, db)
}

func (f *FakeOrchestrator) DisableExternalAccess(ctx context.Context, db *model.ManagedDatabase) error {
	if f.DisableExternalAccessFn != nil {
		return f.DisableExternalAccessFn(ctx, db)
	}
	return f.NoopOrchestrator.DisableExternalAccess(ctx, db)
}

func (f *FakeOrchestrator) RunDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error {
	if f.RunDatabaseBackupFn != nil {
		return f.RunDatabaseBackupFn(ctx, db, backupID, transfer)
	}
	return f.NoopOrchestrator.RunDatabaseBackup(ctx, db, backupID, transfer)
}

func (f *FakeOrchestrator) RestoreDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *orchestrator.ObjectTransfer) error {
	if f.RestoreDatabaseBackupFn != nil {
		return f.RestoreDatabaseBackupFn(ctx, db, backupID, transfer)
	}
	return f.NoopOrchestrator.RestoreDatabaseBackup(ctx, db, backupID, transfer)
}

func (f *FakeOrchestrator) GetBackupJobStatus(ctx context.Context, backupID uuid.UUID) string {
	if f.GetBackupJobStatusFn != nil {
		return f.GetBackupJobStatusFn(ctx, backupID)
	}
	return f.NoopOrchestrator.GetBackupJobStatus(ctx, backupID)
}

// ─── CronJobManager overrides ────────────────────────────────────

func (f *FakeOrchestrator) CreateCronJob(ctx context.Context, cj *model.CronJob) error {
	if f.CreateCronJobFn != nil {
		return f.CreateCronJobFn(ctx, cj)
	}
	return f.NoopOrchestrator.CreateCronJob(ctx, cj)
}

func (f *FakeOrchestrator) UpdateCronJob(ctx context.Context, cj *model.CronJob) error {
	if f.UpdateCronJobFn != nil {
		return f.UpdateCronJobFn(ctx, cj)
	}
	return f.NoopOrchestrator.UpdateCronJob(ctx, cj)
}

func (f *FakeOrchestrator) DeleteCronJob(ctx context.Context, cj *model.CronJob) error {
	if f.DeleteCronJobFn != nil {
		return f.DeleteCronJobFn(ctx, cj)
	}
	return f.NoopOrchestrator.DeleteCronJob(ctx, cj)
}

func (f *FakeOrchestrator) SuspendCronJob(ctx context.Context, cj *model.CronJob, suspend bool) error {
	if f.SuspendCronJobFn != nil {
		return f.SuspendCronJobFn(ctx, cj, suspend)
	}
	return f.NoopOrchestrator.SuspendCronJob(ctx, cj, suspend)
}

func (f *FakeOrchestrator) TriggerCronJob(ctx context.Context, cj *model.CronJob) (string, error) {
	if f.TriggerCronJobFn != nil {
		return f.TriggerCronJobFn(ctx, cj)
	}
	return f.NoopOrchestrator.TriggerCronJob(ctx, cj)
}

func (f *FakeOrchestrator) GetJobStatus(ctx context.Context, cj *model.CronJob, jobName string) (string, error) {
	if f.GetJobStatusFn != nil {
		return f.GetJobStatusFn(ctx, cj, jobName)
	}
	return f.NoopOrchestrator.GetJobStatus(ctx, cj, jobName)
}

// ─── StorageManager overrides ────────────────────────────────────

func (f *FakeOrchestrator) CreateVolume(ctx context.Context, opts orchestrator.VolumeOpts) (string, error) {
	if f.CreateVolumeFn != nil {
		return f.CreateVolumeFn(ctx, opts)
	}
	return f.NoopOrchestrator.CreateVolume(ctx, opts)
}

func (f *FakeOrchestrator) DeleteVolume(ctx context.Context, name, namespace string) error {
	if f.DeleteVolumeFn != nil {
		return f.DeleteVolumeFn(ctx, name, namespace)
	}
	return f.NoopOrchestrator.DeleteVolume(ctx, name, namespace)
}

func (f *FakeOrchestrator) ExpandVolume(ctx context.Context, name, namespace, newSize string) error {
	if f.ExpandVolumeFn != nil {
		return f.ExpandVolumeFn(ctx, name, namespace, newSize)
	}
	return f.NoopOrchestrator.ExpandVolume(ctx, name, namespace, newSize)
}

// ─── LogStreamer override ────────────────────────────────────────

func (f *FakeOrchestrator) StreamLogs(ctx context.Context, app *model.Application, opts orchestrator.LogOpts) (io.ReadCloser, error) {
	if f.StreamLogsFn != nil {
		return f.StreamLogsFn(ctx, app, opts)
	}
	return f.NoopOrchestrator.StreamLogs(ctx, app, opts)
}

func (f *FakeOrchestrator) ExecTerminal(ctx context.Context, app *model.Application, opts orchestrator.ExecOpts) (orchestrator.TerminalSession, error) {
	if f.ExecTerminalFn != nil {
		return f.ExecTerminalFn(ctx, app, opts)
	}
	return f.NoopOrchestrator.ExecTerminal(ctx, app, opts)
}

// ─── ClusterManager overrides ────────────────────────────────────

func (f *FakeOrchestrator) GetNodes(ctx context.Context) ([]orchestrator.NodeInfo, error) {
	if f.GetNodesFn != nil {
		return f.GetNodesFn(ctx)
	}
	return f.NoopOrchestrator.GetNodes(ctx)
}

func (f *FakeOrchestrator) GetClusterMetrics(ctx context.Context) (*orchestrator.ClusterMetrics, error) {
	if f.GetClusterMetricsFn != nil {
		return f.GetClusterMetricsFn(ctx)
	}
	return f.NoopOrchestrator.GetClusterMetrics(ctx)
}

func (f *FakeOrchestrator) GetNodeMetrics(ctx context.Context) ([]orchestrator.NodeMetrics, error) {
	if f.GetNodeMetricsFn != nil {
		return f.GetNodeMetricsFn(ctx)
	}
	return f.NoopOrchestrator.GetNodeMetrics(ctx)
}

func (f *FakeOrchestrator) GetAllPods(ctx context.Context) ([]orchestrator.PodInfo, error) {
	if f.GetAllPodsFn != nil {
		return f.GetAllPodsFn(ctx)
	}
	return f.NoopOrchestrator.GetAllPods(ctx)
}

func (f *FakeOrchestrator) GetClusterEvents(ctx context.Context, limit int) ([]orchestrator.ClusterEvent, error) {
	if f.GetClusterEventsFn != nil {
		return f.GetClusterEventsFn(ctx, limit)
	}
	return f.NoopOrchestrator.GetClusterEvents(ctx, limit)
}
