package model

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// WorkerStatus is the lifecycle status of a Worker.
type WorkerStatus string

const (
	WorkerStatusIdle      WorkerStatus = "idle"
	WorkerStatusDeploying WorkerStatus = "deploying"
	WorkerStatusLive      WorkerStatus = "live"
	WorkerStatusError     WorkerStatus = "error"
	WorkerStatusDraining  WorkerStatus = "draining" // delete teardown in progress
)

// WorkerDeploymentStatus is the lifecycle status of a single Worker deployment.
type WorkerDeploymentStatus string

const (
	WorkerDeployQueued WorkerDeploymentStatus = "queued"
	// WorkerDeployNeedsConfirmation pauses a deploy after detecting that an R2
	// bucket referenced by the wrangler config already exists in the account.
	// The user must confirm reuse (it may belong to another app) before the
	// deploy proceeds. See WorkerDeployment.R2Pending.
	WorkerDeployNeedsConfirmation WorkerDeploymentStatus = "needs_confirmation"
	WorkerDeployDeploying         WorkerDeploymentStatus = "deploying"
	WorkerDeploySuccess           WorkerDeploymentStatus = "success"
	WorkerDeployFailed            WorkerDeploymentStatus = "failed"
	WorkerDeployCancelled         WorkerDeploymentStatus = "cancelled"
)

// WorkerR2Bucket describes an R2 bucket referenced by a worker's wrangler config
// that already exists in the Cloudflare account, surfaced for one-time user
// confirmation before the deploy reuses it.
type WorkerR2Bucket struct {
	Name  string `json:"name"`
	Empty bool   `json:"empty"`
}

// WorkerRuntime holds the Cloudflare resource identifiers discovered after a
// successful `wrangler deploy`. Persisted incrementally so the detail page can
// surface the live URL even between deploys.
type WorkerRuntime struct {
	ScriptName   string   `json:"script_name,omitempty"`
	DeployedURL  string   `json:"deployed_url,omitempty"`
	Routes       []string `json:"routes,omitempty"`
	LastDeployID string   `json:"last_deploy_id,omitempty"`
}

// Worker is a Cloudflare Worker deployed from a git repo via `wrangler deploy`
// in an in-cluster build pod (no Kubernetes workload object is created).
type Worker struct {
	BaseModel `bun:"table:workers,alias:wk"`

	ProjectID uuid.UUID `bun:"project_id,notnull,type:uuid" json:"project_id"`
	Project   *Project  `bun:"rel:belongs-to,join:project_id=id" json:"-"`

	Name        string `bun:"name,notnull" json:"name"`
	Description string `bun:"description,default:''" json:"description"`

	// Git source
	GitRepo       string     `bun:"git_repo" json:"git_repo"`
	GitBranch     string     `bun:"git_branch,default:'main'" json:"git_branch"`
	GitProviderID *uuid.UUID `bun:"git_provider_id,type:uuid" json:"git_provider_id,omitempty"`

	// RootDirectory is the monorepo subdir holding the wrangler project.
	RootDirectory string `bun:"root_directory,default:'.'" json:"root_directory"`
	// WranglerConfig is the path (relative to RootDirectory) to wrangler.toml.
	WranglerConfig string `bun:"wrangler_config,default:'wrangler.toml'" json:"wrangler_config"`

	// Build configuration for installing dependencies before `wrangler deploy`.
	PackageManager string            `bun:"package_manager,default:'auto'" json:"package_manager"` // auto | npm | pnpm
	InstallCommand string            `bun:"install_command,default:''" json:"install_command"`     // empty = inferred default
	BuildCommand   string            `bun:"build_command,default:''" json:"build_command"`         // empty = skip build step
	DeployCommand  string            `bun:"deploy_command,default:''" json:"deploy_command"`       // empty = default "wrangler deploy"
	BuildEnvVars   map[string]string `bun:"build_env_vars,type:jsonb,default:'{}'" json:"build_env_vars"`

	// R2ConfirmedBuckets are R2 bucket names the user has explicitly approved for
	// reuse. A deploy auto-creates missing buckets, but pauses for confirmation
	// when it finds a pre-existing bucket whose name isn't listed here (it could
	// belong to another app).
	R2ConfirmedBuckets []string `bun:"r2_confirmed_buckets,type:jsonb,default:'[]'" json:"r2_confirmed_buckets"`

	// Cloud account (shared_resource with provider cloudflare).
	CloudAccountID *uuid.UUID `bun:"cloud_account_id,type:uuid" json:"cloud_account_id,omitempty"`

	// Runtime state (Cloudflare script identifiers), persisted incrementally.
	Runtime *WorkerRuntime `bun:"runtime,type:jsonb,default:'{}'" json:"runtime"`

	Status WorkerStatus `bun:"status,default:'idle'" json:"status"`

	// Created early to mirror Pages; UNWIRED until auto-deploy lands.
	WebhookSecret string `bun:"webhook_secret,default:''" json:"-"`
	AutoDeploy    bool   `bun:"auto_deploy,default:false" json:"auto_deploy"`
}

var _ bun.AfterScanRowHook = (*Worker)(nil)

func (w *Worker) AfterScanRow(ctx context.Context) error {
	if w.Runtime == nil {
		w.Runtime = &WorkerRuntime{}
	}
	if w.BuildEnvVars == nil {
		w.BuildEnvVars = map[string]string{}
	}
	if w.R2ConfirmedBuckets == nil {
		w.R2ConfirmedBuckets = []string{}
	}
	return nil
}

// WorkerDeployment records a single deploy (clone + install + wrangler deploy).
type WorkerDeployment struct {
	BaseModel `bun:"table:worker_deployments,alias:wkd"`

	WorkerID uuid.UUID `bun:"worker_id,notnull,type:uuid" json:"worker_id"`
	Worker   *Worker   `bun:"rel:belongs-to,join:worker_id=id" json:"-"`

	Status      WorkerDeploymentStatus `bun:"status,default:'queued'" json:"status"`
	CommitSHA   string                 `bun:"commit_sha,default:''" json:"commit_sha"`
	DeployLog   string                 `bun:"deploy_log,type:text,default:''" json:"deploy_log"`
	ProviderRef string                 `bun:"provider_ref,default:''" json:"provider_ref"` // deployed script name
	TriggerType string                 `bun:"trigger_type,default:'manual'" json:"trigger_type"`

	// R2Pending lists the pre-existing R2 buckets awaiting confirmation when
	// Status is needs_confirmation; empty otherwise.
	R2Pending []WorkerR2Bucket `bun:"r2_pending,type:jsonb,default:'[]'" json:"r2_pending,omitempty"`

	StartedAt  *time.Time `bun:"started_at" json:"started_at"`
	FinishedAt *time.Time `bun:"finished_at" json:"finished_at"`
}
