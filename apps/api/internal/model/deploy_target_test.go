package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDeployTargetID(t *testing.T) {
	assert.Equal(t, "00000000-0000-4000-8000-000000000001", DefaultDeployTargetID.String())
}

func TestDeployTargetKind(t *testing.T) {
	assert.Equal(t, DeployTargetKind("k3s"), DeployTargetK3s)
}
