package model

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// PageStatus is the lifecycle status of a Page.
type PageStatus string

const (
	PageStatusIdle      PageStatus = "idle"
	PageStatusDeploying PageStatus = "deploying"
	PageStatusLive      PageStatus = "live"
	PageStatusError     PageStatus = "error"
	PageStatusDraining  PageStatus = "draining" // Phase 4 (delete)
)

// PageProvider identifies the hosting backend for a Page.
type PageProvider string

const (
	PageProviderAWSCloudFront   PageProvider = "aws_cloudfront"
	PageProviderCloudflarePages PageProvider = "cloudflare_pages"
)

// PageDeploymentStatus is the lifecycle status of a single Page deployment.
type PageDeploymentStatus string

const (
	PageDeployQueued    PageDeploymentStatus = "queued"
	PageDeployDeploying PageDeploymentStatus = "deploying"
	PageDeploySuccess   PageDeploymentStatus = "success"
	PageDeployFailed    PageDeploymentStatus = "failed"
	PageDeployCancelled PageDeploymentStatus = "cancelled"
)

// PageValidationRecord is the ACM DNS validation CNAME ACM requires.
type PageValidationRecord struct {
	Name  string `json:"name,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// PageRuntime holds the cloud resource identifiers created for a Page. It is
// persisted incrementally as each AWS resource is provisioned so that a
// re-deploy after a mid-provision failure can resume idempotently instead of
// duplicating resources.
type PageRuntime struct {
	BucketName      string `json:"bucket_name,omitempty"`
	DistributionID  string `json:"distribution_id,omitempty"`
	DistributionARN string `json:"distribution_arn,omitempty"` // needed to (re)apply the bucket policy idempotently
	OACID           string `json:"oac_id,omitempty"`           // CloudFront Origin Access Control ID
	DefaultURL      string `json:"default_url,omitempty"`

	// Custom domain + ACM (certificate always in us-east-1 for CloudFront).
	CertificateARN   string                `json:"certificate_arn,omitempty"`
	CertStatus       string                `json:"cert_status,omitempty"` // PENDING_VALIDATION | ISSUED | ...
	ValidationRecord *PageValidationRecord `json:"validation_record,omitempty"`
	AliasTarget      string                `json:"alias_target,omitempty"` // CloudFront domain for final DNS alias

	// Cloudflare Pages (provider cloudflare_pages).
	CFProjectName string `json:"cf_project_name,omitempty"`
	CFProjectID   string `json:"cf_project_id,omitempty"`
}

// Page is a static site deployed to cloud CDN + object storage (no Kubernetes).
type Page struct {
	BaseModel `bun:"table:pages,alias:pg"`

	ProjectID uuid.UUID `bun:"project_id,notnull,type:uuid" json:"project_id"`
	Project   *Project  `bun:"rel:belongs-to,join:project_id=id" json:"-"`

	Name        string `bun:"name,notnull" json:"name"`
	Description string `bun:"description,default:''" json:"description"`

	// Git source
	GitRepo       string     `bun:"git_repo" json:"git_repo"`
	GitBranch     string     `bun:"git_branch,default:'main'" json:"git_branch"`
	GitProviderID *uuid.UUID `bun:"git_provider_id,type:uuid" json:"git_provider_id,omitempty"`
	// PublishPath is the folder in the repo whose contents are synced to the
	// bucket root. Default "." = repo root. Used when BuildEnabled is false
	// (pre-built static files committed to the repo).
	PublishPath string `bun:"publish_path,default:'.'" json:"publish_path"`

	// Build configuration. When BuildEnabled is true, the site is built in an
	// in-cluster Kubernetes build pod (npm/pnpm) and the contents of
	// RootDirectory/OutputDir are synced instead of PublishPath.
	BuildEnabled   bool              `bun:"build_enabled,default:false" json:"build_enabled"`
	PackageManager string            `bun:"package_manager,default:'auto'" json:"package_manager"` // auto | npm | pnpm
	InstallCommand string            `bun:"install_command,default:''" json:"install_command"`     // empty = inferred default
	BuildCommand   string            `bun:"build_command,default:''" json:"build_command"`         // empty = inferred "<pm> run build"
	OutputDir      string            `bun:"output_dir,default:''" json:"output_dir"`               // build output folder, e.g. "dist"
	RootDirectory  string            `bun:"root_directory,default:'.'" json:"root_directory"`      // monorepo subdir holding package.json
	NodeVersion    string            `bun:"node_version,default:''" json:"node_version"`           // optional; default in code
	BuildEnvVars   map[string]string `bun:"build_env_vars,type:jsonb,default:'{}'" json:"build_env_vars"`

	// Provider + cloud account
	Provider       PageProvider `bun:"provider,default:'aws_cloudfront'" json:"provider"`
	CloudAccountID *uuid.UUID   `bun:"cloud_account_id,type:uuid" json:"cloud_account_id,omitempty"`
	Region         string       `bun:"region,default:'us-east-1'" json:"region"`

	// Custom domain (optional). ACM cert is always us-east-1; DNS may use a
	// different cloud_account than CloudFront/S3.
	CustomDomain string     `bun:"custom_domain,default:''" json:"custom_domain"`
	ManageDNS    bool       `bun:"manage_dns,default:false" json:"manage_dns"`
	DNSAccountID *uuid.UUID `bun:"dns_account_id,type:uuid" json:"dns_account_id,omitempty"`
	DNSZoneID    string     `bun:"dns_zone_id,default:''" json:"dns_zone_id"`

	// Runtime state (AWS resource IDs), persisted incrementally.
	Runtime *PageRuntime `bun:"runtime,type:jsonb,default:'{}'" json:"runtime"`

	Status PageStatus `bun:"status,default:'idle'" json:"status"`

	// Phase 2 — created early to avoid an extra migration, but UNWIRED until then.
	WebhookSecret string `bun:"webhook_secret,default:''" json:"-"`
	AutoDeploy    bool   `bun:"auto_deploy,default:false" json:"auto_deploy"`
}

var _ bun.AfterScanRowHook = (*Page)(nil)

func (p *Page) AfterScanRow(ctx context.Context) error {
	if p.Runtime == nil {
		p.Runtime = &PageRuntime{}
	}
	if p.BuildEnvVars == nil {
		p.BuildEnvVars = map[string]string{}
	}
	return nil
}

// PageDeployment records a single deploy (clone + sync) of a Page.
type PageDeployment struct {
	BaseModel `bun:"table:page_deployments,alias:pgd"`

	PageID uuid.UUID `bun:"page_id,notnull,type:uuid" json:"page_id"`
	Page   *Page     `bun:"rel:belongs-to,join:page_id=id" json:"-"`

	Status PageDeploymentStatus `bun:"status,default:'queued'" json:"status"`
	// PublishPath snapshots the normalized folder synced by this deploy. It is
	// part of the dedup key alongside CommitSHA: changing publish_path without a
	// new commit must still re-sync, otherwise the CDN keeps serving the old
	// folder behind a green status.
	PublishPath string `bun:"publish_path,default:'.'" json:"publish_path"`
	CommitSHA   string `bun:"commit_sha,default:''" json:"commit_sha"`
	DeployLog   string `bun:"deploy_log,type:text,default:''" json:"deploy_log"`
	ProviderRef string `bun:"provider_ref,default:''" json:"provider_ref"` // e.g. CF invalidation ID
	TriggerType string `bun:"trigger_type,default:'manual'" json:"trigger_type"`

	StartedAt  *time.Time `bun:"started_at" json:"started_at"`
	FinishedAt *time.Time `bun:"finished_at" json:"finished_at"`
}
