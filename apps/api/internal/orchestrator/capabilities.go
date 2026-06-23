package orchestrator

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

// LogStreamer provides real-time log streaming.
type LogStreamer interface {
	StreamLogs(ctx context.Context, app *model.Application, opts LogOpts) (io.ReadCloser, error)
	StreamPodLogs(ctx context.Context, app *model.Application, podName string, opts LogOpts) (io.ReadCloser, error)
}

// Execer provides interactive terminal access to containers.
type Execer interface {
	ExecTerminal(ctx context.Context, app *model.Application, opts ExecOpts) (TerminalSession, error)
}

// TerminalExec is the legacy name for Execer.
type TerminalExec = Execer

// Builder handles image building from source code.
type Builder interface {
	Build(ctx context.Context, app *model.Application, opts BuildOpts) (*BuildResult, error)
	EnsureRegistry(ctx context.Context) error
	GetBuildLogs(ctx context.Context, jobName, namespace string) (io.ReadCloser, error)
	ClearBuildCache(ctx context.Context, app *model.Application) error
	CancelBuild(ctx context.Context, app *model.Application) error
}

// BuildManager is the legacy name for Builder.
type BuildManager = Builder

// StaticSiteBuilder builds a static site (e.g. React/Vue) from a git repo in an
// in-cluster build pod and returns the built output on the local filesystem,
// ready to be synced to object storage. Unlike Builder it does not produce a
// container image — it transforms source into a directory of static files.
type StaticSiteBuilder interface {
	BuildStatic(ctx context.Context, opts StaticBuildOpts) (*StaticBuildResult, error)
}

// StaticBuildOpts configures a static-site build.
type StaticBuildOpts struct {
	GitRepo   string
	GitBranch string
	GitToken  string
	// PageID scopes build pod/namespace names to a single Page.
	PageID string
	// RootDirectory is the subdirectory (relative to the repo root) that holds
	// package.json; the install/build commands run there. Default ".".
	RootDirectory string
	// PackageManager is "auto" (infer from lockfile), "npm", or "pnpm".
	PackageManager string
	// InstallCommand / BuildCommand override the inferred defaults when set.
	InstallCommand string
	BuildCommand   string
	// OutputDir is the build output folder (relative to RootDirectory) whose
	// contents are returned, e.g. "dist" or "build".
	OutputDir string
	// BuildEnvVars are injected into the build container as environment.
	BuildEnvVars map[string]string
	// NodeImage overrides the default node build image.
	NodeImage string
	OnLog     LogCallback
}

// StaticBuildResult is the outcome of a static-site build.
type StaticBuildResult struct {
	// FilesDir is a local directory containing the built static files. The
	// caller must invoke Cleanup when done.
	FilesDir string
	// CommitSHA is the HEAD commit that was built.
	CommitSHA string
	// Cleanup removes FilesDir; always call it when done.
	Cleanup  func()
	Logs     string
	Duration time.Duration
}

// WorkerBuilder clones a git repo and runs `wrangler deploy` in an in-cluster
// build pod to deploy a Cloudflare Worker. Unlike StaticSiteBuilder it does not
// return files — the deploy is the terminal action (the script is pushed to
// Cloudflare directly from the pod). DeleteWorker tears the script back down.
type WorkerBuilder interface {
	BuildWorker(ctx context.Context, opts WorkerBuildOpts) (*WorkerBuildResult, error)
	DeleteWorker(ctx context.Context, opts WorkerDeleteOpts) (*WorkerBuildResult, error)
}

// WorkerBuildOpts configures a `wrangler deploy` build.
type WorkerBuildOpts struct {
	GitRepo   string
	GitBranch string
	GitToken  string
	// WorkerID scopes build pod/namespace names to a single Worker.
	WorkerID string
	// RootDirectory is the subdirectory (relative to the repo root) that holds
	// the wrangler project; install + deploy run there. Default ".".
	RootDirectory string
	// WranglerConfig is the path to wrangler.toml relative to RootDirectory.
	WranglerConfig string
	// PackageManager is "auto" (infer from lockfile), "npm", or "pnpm".
	PackageManager string
	// InstallCommand overrides the inferred install command when set.
	InstallCommand string
	// BuildCommand runs after install and before deploy when set; empty skips
	// the build step.
	BuildCommand string
	// DeployCommand overrides the default `wrangler deploy` when set.
	DeployCommand string
	// BuildEnvVars are injected into the build container as environment.
	BuildEnvVars map[string]string
	// R2ConfirmedBuckets are R2 bucket names the user has approved for reuse.
	// The build pod auto-creates missing buckets but gates the deploy when it
	// finds a pre-existing bucket whose name isn't in this list.
	R2ConfirmedBuckets []string
	// CFAPIToken / CFAccountID authenticate wrangler against Cloudflare. They
	// are injected via a K8s Secret (secretKeyRef), never inline in the pod spec.
	CFAPIToken  string
	CFAccountID string
	OnLog       LogCallback
}

// WorkerDeleteOpts configures a `wrangler delete` teardown.
type WorkerDeleteOpts struct {
	GitRepo        string
	GitBranch      string
	GitToken       string
	WorkerID       string
	RootDirectory  string
	WranglerConfig string
	PackageManager string
	InstallCommand string
	// ScriptName overrides the wrangler.toml name for the delete target.
	ScriptName  string
	CFAPIToken  string
	CFAccountID string
	OnLog       LogCallback
}

// WorkerBuildResult is the outcome of a wrangler deploy/delete run.
type WorkerBuildResult struct {
	// CommitSHA is the HEAD commit that was deployed.
	CommitSHA string
	// ScriptName is the Cloudflare Worker script name (from deploy output or
	// wrangler.toml).
	ScriptName string
	// DeployedURL is the *.workers.dev URL parsed from the deploy output.
	DeployedURL string
	// DeployID is the Cloudflare deployment/version ID parsed from the output.
	DeployID string
	// NeedsR2Confirmation is true when the build pod stopped before deploying
	// because one or more R2 buckets referenced by the wrangler config already
	// exist and weren't in WorkerBuildOpts.R2ConfirmedBuckets. PendingR2Buckets
	// lists them; nothing was deployed.
	NeedsR2Confirmation bool
	PendingR2Buckets    []model.WorkerR2Bucket
	Logs                string
	Duration            time.Duration
}

// SecretSink handles runtime secret lifecycle for workloads.
type SecretSink interface {
	EnsureSecret(ctx context.Context, app *model.Application, secrets map[string]string) error
	// EnsureImagePullSecret creates/updates a dockerconfigjson pull secret for the app.
	// An empty document deletes any existing pull secret.
	EnsureImagePullSecret(ctx context.Context, app *model.Application, dockerConfigJSON []byte) error
}

// SecretManager is the legacy name for SecretSink.
type SecretManager = SecretSink

// IngressBinder handles domain routing and TLS.
type IngressBinder interface {
	CreateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error
	UpdateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error
	DeleteIngress(ctx context.Context, domain *model.Domain) error
	DeleteIngressByName(ctx context.Context, app *model.Application, name string) error
	IngressName(app *model.Application, host string) string
	LegacyIngressName(app *model.Application, host string) string
	GetIngressStatus(ctx context.Context, domain *model.Domain, app *model.Application) (*IngressStatus, error)
	GetCertExpiry(ctx context.Context, domain *model.Domain, app *model.Application) (*time.Time, error)
	EnsurePanelIngress(ctx context.Context, domain, httpsEmail string) error
	DeletePanelIngress(ctx context.Context) error
}

// IngressManager is the legacy name for IngressBinder.
type IngressManager = IngressBinder

// VolumeProvider handles persistent volumes.
type VolumeProvider interface {
	CreateVolume(ctx context.Context, opts VolumeOpts) (string, error)
	DeleteVolume(ctx context.Context, name, namespace string) error
	ExpandVolume(ctx context.Context, name, namespace, newSize string) error
}

// StorageManager is the legacy name for VolumeProvider.
type StorageManager = VolumeProvider

// DatabaseManager handles managed database lifecycle.
type DatabaseManager interface {
	DeployDatabase(ctx context.Context, db *model.ManagedDatabase) error
	DeleteDatabase(ctx context.Context, db *model.ManagedDatabase) error
	GetDatabaseStatus(ctx context.Context, db *model.ManagedDatabase) (*AppStatus, error)
	GetDatabaseCredentials(ctx context.Context, db *model.ManagedDatabase) (*DatabaseCredentials, error)
	GetDatabasePods(ctx context.Context, db *model.ManagedDatabase) ([]PodInfo, error)
	RunDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *ObjectTransfer) error
	RestoreDatabaseBackup(ctx context.Context, db *model.ManagedDatabase, backupID uuid.UUID, transfer *ObjectTransfer) error
	GetBackupJobStatus(ctx context.Context, backupID uuid.UUID) string
	GetRestoreJobStatus(ctx context.Context, backupID uuid.UUID) string
	EnableExternalAccess(ctx context.Context, db *model.ManagedDatabase) (int32, error)
	DisableExternalAccess(ctx context.Context, db *model.ManagedDatabase) error
}

// CronManager handles scheduled job lifecycle.
type CronManager interface {
	CreateCronJob(ctx context.Context, cj *model.CronJob) error
	UpdateCronJob(ctx context.Context, cj *model.CronJob) error
	DeleteCronJob(ctx context.Context, cj *model.CronJob) error
	SuspendCronJob(ctx context.Context, cj *model.CronJob, suspend bool) error
	TriggerCronJob(ctx context.Context, cj *model.CronJob) (string, error)
	GetJobStatus(ctx context.Context, cj *model.CronJob, jobName string) (string, error)
}

// CronJobManager is the legacy name for CronManager.
type CronJobManager = CronManager
