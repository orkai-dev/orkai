package k3s

import (
	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

// Target wraps the K3s Orchestrator as a DeployTarget.
type Target struct {
	*Orchestrator
	id uuid.UUID
}

// NewTarget wraps an existing K3s orchestrator with deploy-target metadata.
func NewTarget(orch *Orchestrator, id uuid.UUID) *Target {
	return &Target{Orchestrator: orch, id: id}
}

func (t *Target) ID() uuid.UUID                     { return t.id }
func (t *Target) Kind() string                      { return "k3s" }
func (t *Target) Capabilities() orchestrator.CapSet { return orchestrator.AllCapabilities() }

var (
	_ orchestrator.DeployTarget        = (*Target)(nil)
	_ orchestrator.KubernetesInspector = (*Target)(nil)
	_ orchestrator.Deployer            = (*Target)(nil)
	_ orchestrator.Builder             = (*Target)(nil)
	_ orchestrator.StaticSiteBuilder   = (*Target)(nil)
	_ orchestrator.WorkerBuilder       = (*Target)(nil)
	_ orchestrator.DatabaseManager     = (*Target)(nil)
	_ orchestrator.IngressBinder       = (*Target)(nil)
	_ orchestrator.LogStreamer         = (*Target)(nil)
	_ orchestrator.Execer              = (*Target)(nil)
	_ orchestrator.SecretSink          = (*Target)(nil)
	_ orchestrator.CronManager         = (*Target)(nil)
	_ orchestrator.VolumeProvider      = (*Target)(nil)
)
