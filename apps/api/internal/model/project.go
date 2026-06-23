package model

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Environment classifies where a project's services run.
type Environment string

const (
	EnvProd        Environment = "prod"
	EnvTesting     Environment = "testing"
	EnvSandbox     Environment = "sandbox"
	EnvQA          Environment = "qa"
	EnvPOC         Environment = "poc"
	EnvDevelopment Environment = "development"
)

// ValidEnvironments lists every accepted Environment value.
var ValidEnvironments = []Environment{
	EnvProd, EnvTesting, EnvSandbox, EnvQA, EnvPOC, EnvDevelopment,
}

// IsValid reports whether the environment is one of the accepted values.
func (e Environment) IsValid() bool {
	for _, v := range ValidEnvironments {
		if e == v {
			return true
		}
	}
	return false
}

// ResourceQuotaConfig defines project-level resource quotas.
type ResourceQuotaConfig struct {
	CPULimit     string `json:"cpu_limit,omitempty"`     // e.g. "4000m"
	MemLimit     string `json:"mem_limit,omitempty"`     // e.g. "8Gi"
	PodLimit     int    `json:"pod_limit,omitempty"`     // max pods
	PVCLimit     int    `json:"pvc_limit,omitempty"`     // max PVCs
	StorageLimit string `json:"storage_limit,omitempty"` // e.g. "50Gi"
}

// Project is a logical grouping of applications within an organization.
type Project struct {
	BaseModel `bun:"table:projects,alias:p"`

	OrgID        uuid.UUID     `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Organization *Organization `bun:"rel:belongs-to,join:org_id=id" json:"-"`

	TeamID uuid.UUID `bun:"team_id,notnull,type:uuid" json:"team_id"`
	Team   *Team     `bun:"rel:belongs-to,join:team_id=id" json:"team,omitempty"`

	Name        string `bun:"name,notnull" json:"name"`
	Namespace   string `bun:"namespace" json:"namespace"`
	Description string `bun:"description" json:"description"`

	// Metadata
	Environment Environment `bun:"environment,default:'development'" json:"environment"`

	// Project environment (shared by all services)
	EnvVars map[string]string `bun:"env_vars,type:jsonb,default:'{}'" json:"env_vars"`

	// Service account
	ServiceAccount string `bun:"service_account,default:''" json:"service_account"`

	// Resource controls
	ResourceQuota        *ResourceQuotaConfig `bun:"resource_quota,type:jsonb,default:'{}'" json:"resource_quota"`
	NetworkPolicyEnabled bool                 `bun:"network_policy_enabled,default:false" json:"network_policy_enabled"`

	// Relations
	Applications []Application `bun:"rel:has-many,join:id=project_id" json:"-"`
	CronJobs     []CronJob     `bun:"rel:has-many,join:id=project_id" json:"-"`
	Pages        []Page        `bun:"rel:has-many,join:id=project_id" json:"-"`
}

var _ bun.AfterScanRowHook = (*Project)(nil)

func (p *Project) AfterScanRow(ctx context.Context) error {
	if p.EnvVars == nil {
		p.EnvVars = map[string]string{}
	}
	if p.ResourceQuota == nil {
		p.ResourceQuota = &ResourceQuotaConfig{}
	}
	return nil
}
