package orchestrator

import (
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetRegistryDefault(t *testing.T) {
	id := uuid.New()
	target := NewNoopTarget(id, nil)
	reg, err := NewTargetRegistry(uuid.Nil, target)
	require.NoError(t, err)
	assert.Equal(t, target, reg.Default())
}

func TestTargetRegistryFor(t *testing.T) {
	id := uuid.New()
	target := NewNoopTarget(id, nil)
	reg, err := NewTargetRegistry(id, target)
	require.NoError(t, err)

	got, err := reg.For(nil)
	require.NoError(t, err)
	assert.Equal(t, target, got)

	app := &model.Application{TargetID: &id}
	got, err = reg.For(app)
	require.NoError(t, err)
	assert.Equal(t, target, got)

	unknown := uuid.New()
	app.TargetID = &unknown
	_, err = reg.For(app)
	require.Error(t, err)
	var notFound ErrTargetNotFound
	require.ErrorAs(t, err, &notFound)
}

func TestTargetRegistryGetNilUsesDefault(t *testing.T) {
	id := uuid.New()
	target := NewNoopTarget(id, nil)
	reg, err := NewTargetRegistry(id, target)
	require.NoError(t, err)

	got, err := reg.Get(uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}

func TestTargetRegistryErrors(t *testing.T) {
	_, err := NewTargetRegistry(uuid.Nil)
	require.Error(t, err)

	id := uuid.New()
	target := NewNoopTarget(id, nil)
	_, err = NewTargetRegistry(uuid.New(), target)
	require.Error(t, err)

	reg, err := NewTargetRegistry(id, target)
	require.NoError(t, err)
	err = reg.Register(target)
	require.Error(t, err)
}

func TestTargetRegistryRegister(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	t1 := NewNoopTarget(id1, nil)
	reg, err := NewTargetRegistry(id1, t1)
	require.NoError(t, err)

	t2 := NewNoopTarget(id2, nil)
	require.NoError(t, reg.Register(t2))

	got, err := reg.Get(id2)
	require.NoError(t, err)
	assert.Equal(t, t2, got)
}

type shutdownSpyTarget struct {
	*NoopTarget
	shutdowns int
}

func (s *shutdownSpyTarget) Shutdown() {
	s.shutdowns++
}

func TestTargetRegistryShutdownAllTargets(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	spy1 := &shutdownSpyTarget{NoopTarget: NewNoopTarget(id1, nil)}
	spy2 := &shutdownSpyTarget{NoopTarget: NewNoopTarget(id2, nil)}
	reg, err := NewTargetRegistry(id1, spy1)
	require.NoError(t, err)
	require.NoError(t, reg.Register(spy2))

	reg.Shutdown()

	assert.Equal(t, 1, spy1.shutdowns)
	assert.Equal(t, 1, spy2.shutdowns)
}
