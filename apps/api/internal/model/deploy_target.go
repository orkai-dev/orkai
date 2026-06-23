package model

import (
	"github.com/google/uuid"
)

// DefaultDeployTargetID is the seeded in-cluster K3s target.
var DefaultDeployTargetID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

// DeployTargetKind identifies a deploy-target implementation.
type DeployTargetKind string

const (
	DeployTargetK3s DeployTargetKind = "k3s"
)

// DeployTargetConfig holds per-target cluster settings (storage defaults, etc.).
type DeployTargetConfig struct {
	DefaultStorageClass   string   `json:"default_storage_class,omitempty"`
	AllowedStorageClasses []string `json:"allowed_storage_classes,omitempty"`
}

// DeployTarget is a persisted deploy destination (cluster, cloud, etc.).
type DeployTarget struct {
	BaseModel `bun:"table:deploy_targets,alias:dt"`

	OrgID        *uuid.UUID         `bun:"org_id,type:uuid" json:"org_id,omitempty"`
	Kind         DeployTargetKind   `bun:"kind,notnull" json:"kind"`
	Region       string             `bun:"region,default:''" json:"region"`
	Capabilities []string           `bun:"capabilities,type:jsonb,default:'[]'" json:"capabilities"`
	Config       DeployTargetConfig `bun:"config,type:jsonb,default:'{}'" json:"config"`
	IsDefault    bool               `bun:"is_default,default:false" json:"is_default"`
}
