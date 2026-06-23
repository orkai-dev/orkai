package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	raw, prefix, hash, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(raw, APIKeyPrefix))
	assert.NotEmpty(t, prefix)
	assert.Len(t, prefix, APIKeyDisplayPrefixLen)
	assert.Equal(t, HashAPIKey(raw), hash)

	raw2, _, hash2, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, raw, raw2)
	assert.NotEqual(t, hash, hash2)
}

func TestHashAPIKeyDeterministic(t *testing.T) {
	raw := "ork_testkey"
	assert.Equal(t, HashAPIKey(raw), HashAPIKey(raw))
}
