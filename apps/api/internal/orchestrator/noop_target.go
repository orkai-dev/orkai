package orchestrator

import (
	"log/slog"

	"github.com/google/uuid"
)

// NoopTarget wraps NoopOrchestrator as a DeployTarget for dev without K3s.
type NoopTarget struct {
	*NoopOrchestrator
	id uuid.UUID
}

// NewNoopTarget creates a noop deploy target with the given id.
func NewNoopTarget(id uuid.UUID, logger *slog.Logger) *NoopTarget {
	if id == uuid.Nil {
		id = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	}
	return &NoopTarget{
		NoopOrchestrator: NewNoop(logger),
		id:               id,
	}
}

func (n *NoopTarget) ID() uuid.UUID        { return n.id }
func (n *NoopTarget) Kind() string         { return "noop" }
func (n *NoopTarget) Capabilities() CapSet { return AllCapabilities() }

var _ DeployTarget = (*NoopTarget)(nil)
