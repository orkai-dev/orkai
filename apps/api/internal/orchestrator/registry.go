package orchestrator

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

// TargetRegistry resolves deploy targets by id or application.
type TargetRegistry struct {
	defaultID uuid.UUID
	byID      map[uuid.UUID]DeployTarget
}

// NewTargetRegistry registers targets and picks the first as default when
// defaultID is zero.
func NewTargetRegistry(defaultID uuid.UUID, targets ...DeployTarget) (*TargetRegistry, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("target registry requires at least one target")
	}
	byID := make(map[uuid.UUID]DeployTarget, len(targets))
	for _, t := range targets {
		if t == nil {
			return nil, fmt.Errorf("target registry: nil target")
		}
		id := t.ID()
		if id == uuid.Nil {
			return nil, fmt.Errorf("target registry: target %q has nil id", t.Kind())
		}
		if _, exists := byID[id]; exists {
			return nil, fmt.Errorf("target registry: duplicate target id %s", id)
		}
		byID[id] = t
	}
	if defaultID == uuid.Nil {
		defaultID = targets[0].ID()
	}
	if _, ok := byID[defaultID]; !ok {
		return nil, fmt.Errorf("target registry: default target %s not registered", defaultID)
	}
	return &TargetRegistry{defaultID: defaultID, byID: byID}, nil
}

// Default returns the default deploy target.
func (r *TargetRegistry) Default() DeployTarget {
	return r.byID[r.defaultID]
}

// Get returns the target for id or ErrTargetNotFound.
func (r *TargetRegistry) Get(id uuid.UUID) (DeployTarget, error) {
	if id == uuid.Nil {
		return r.Default(), nil
	}
	t, ok := r.byID[id]
	if !ok {
		return nil, ErrTargetNotFound{TargetID: id}
	}
	return t, nil
}

// For resolves the deploy target for an application.
// Falls back to Default when app is nil or TargetID is nil.
func (r *TargetRegistry) For(app *model.Application) (DeployTarget, error) {
	if app == nil || app.TargetID == nil {
		return r.Default(), nil
	}
	return r.Get(*app.TargetID)
}

// Register adds a target after construction (mainly for tests).
func (r *TargetRegistry) Register(t DeployTarget) error {
	if t == nil {
		return fmt.Errorf("cannot register nil target")
	}
	id := t.ID()
	if id == uuid.Nil {
		return fmt.Errorf("target %q has nil id", t.Kind())
	}
	if _, exists := r.byID[id]; exists {
		return fmt.Errorf("target id %s already registered", id)
	}
	r.byID[id] = t
	return nil
}

// Shutdown stops background work on all registered targets that support it
// (e.g. K3s orchestrator).
func (r *TargetRegistry) Shutdown() {
	for _, t := range r.byID {
		if sh, ok := t.(interface{ Shutdown() }); ok {
			sh.Shutdown()
		}
	}
}

// ErrTargetNotFound is returned when a target id is unknown.
type ErrTargetNotFound struct {
	TargetID uuid.UUID
}

func (e ErrTargetNotFound) Error() string {
	return fmt.Sprintf("deploy target not found: %s", e.TargetID)
}
