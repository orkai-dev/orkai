package providers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECRShortLived(t *testing.T) {
	assert.True(t, newECRRegistry().ShortLived())
	assert.Equal(t, "ecr", newECRRegistry().Name())
}

func TestECRDockerAuthInvalidConfig(t *testing.T) {
	_, _, _, err := newECRRegistry().DockerAuth(context.Background(), json.RawMessage(`bad`))
	require.ErrorContains(t, err, "invalid ecr config")
}

func TestECRAuthTokenNoRegion(t *testing.T) {
	_, _, _, err := ecrAuthToken(context.Background(), "", "ak", "sk")
	require.ErrorContains(t, err, "region is required")
}
