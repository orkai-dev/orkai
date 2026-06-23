package orchestrator

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapSet(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var s CapSet
		assert.False(t, s.Has(CapExec))
		assert.Nil(t, s.List())
	})

	t.Run("new and has", func(t *testing.T) {
		s := NewCapSet(CapExec, CapBuild)
		assert.True(t, s.Has(CapExec))
		assert.True(t, s.Has(CapBuild))
		assert.False(t, s.Has(CapVolumes))
		assert.Len(t, s.List(), 2)
	})

	t.Run("add", func(t *testing.T) {
		s := NewCapSet(CapDeploy)
		s.Add(CapLogs)
		assert.True(t, s.Has(CapLogs))
	})
}

func TestAllCapabilities(t *testing.T) {
	all := AllCapabilities()
	for _, c := range []Capability{
		CapDeploy, CapExec, CapVolumes, CapManagedDB, CapBuild,
		CapIngress, CapSecrets, CapCron, CapLogs, CapKubernetes,
	} {
		assert.True(t, all.Has(c), "missing %s", c)
	}
}

func TestAsCapability(t *testing.T) {
	n := NewNoopTarget(uuid.New(), nil)
	ex, err := AsCapability[Execer](n, CapExec)
	require.NoError(t, err)
	assert.NotNil(t, ex)

	_, err = AsCapability[KubernetesInspector](struct{}{}, CapKubernetes)
	require.Error(t, err)
	var capErr ErrCapabilityUnsupported
	require.ErrorAs(t, err, &capErr)
	assert.Equal(t, CapKubernetes, capErr.Capability)
}

func TestRequireKubernetesInspector(t *testing.T) {
	n := NewNoopTarget(uuid.New(), nil)
	ki, err := RequireKubernetesInspector(n)
	require.NoError(t, err)
	assert.NotNil(t, ki)
}
