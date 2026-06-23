package k3s

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orkai-dev/orkai/apps/api/internal/config"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
)

func TestBootstrapTargetRegistryNoKubeconfig(t *testing.T) {
	fs := testsupport.NewFakeStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg, err := BootstrapTargetRegistry(context.Background(), fs, config.K8sConfig{}, logger)
	require.NoError(t, err)
	assert.NotNil(t, reg.Default())
	assert.Equal(t, model.DefaultDeployTargetID, reg.Default().ID())
	assert.True(t, reg.Default().Capabilities().Has(orchestrator.CapDeploy))
}
