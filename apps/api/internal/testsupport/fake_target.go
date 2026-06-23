package testsupport

import (
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

// FakeTarget wraps FakeOrchestrator as a DeployTarget for service tests.
type FakeTarget struct {
	*FakeOrchestrator
	id uuid.UUID
}

// NewFakeTarget returns a FakeTarget with the given id (or default).
func NewFakeTarget(id uuid.UUID) *FakeTarget {
	if id == uuid.Nil {
		id = model.DefaultDeployTargetID
	}
	return &FakeTarget{
		FakeOrchestrator: NewFakeOrchestrator(),
		id:               id,
	}
}

// NewFakeTargetFromOrchestrator wraps an existing FakeOrchestrator as a DeployTarget.
func NewFakeTargetFromOrchestrator(orch *FakeOrchestrator, id uuid.UUID) *FakeTarget {
	if id == uuid.Nil {
		id = model.DefaultDeployTargetID
	}
	return &FakeTarget{FakeOrchestrator: orch, id: id}
}

func (f *FakeTarget) ID() uuid.UUID                     { return f.id }
func (f *FakeTarget) Kind() string                      { return "k3s" }
func (f *FakeTarget) Capabilities() orchestrator.CapSet { return orchestrator.AllCapabilities() }

var _ orchestrator.DeployTarget = (*FakeTarget)(nil)

// NewFakeTargetRegistry returns a registry backed by a FakeTarget.
// Pass an optional FakeOrchestrator to share override hooks across tests.
func NewFakeTargetRegistry(orch ...*FakeOrchestrator) *orchestrator.TargetRegistry {
	var f *FakeOrchestrator
	if len(orch) > 0 && orch[0] != nil {
		f = orch[0]
	} else {
		f = NewFakeOrchestrator()
	}
	t := NewFakeTargetFromOrchestrator(f, model.DefaultDeployTargetID)
	reg, _ := orchestrator.NewTargetRegistry(t.ID(), t)
	return reg
}
