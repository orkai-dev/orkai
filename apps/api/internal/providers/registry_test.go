package providers

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSettings map[string]string

func (f fakeSettings) Get(ctx context.Context, key string) (string, error) {
	return f[key], nil
}

func newTestRegistry() *Registry {
	return New(fakeSettings{}, slog.Default())
}

func TestRegistryGitLookup(t *testing.T) {
	r := newTestRegistry()

	for _, name := range []string{"github", "gitlab", "gitea"} {
		p, err := r.Git(name)
		require.NoError(t, err)
		assert.Equal(t, name, p.Name())
	}

	_, err := r.Git("bitbucket")
	require.Error(t, err)
	var unsupported ErrUnsupportedProvider
	require.True(t, errors.As(err, &unsupported))
	assert.Contains(t, err.Error(), "unsupported provider: bitbucket")
}

func TestRegistryRegistryLookup(t *testing.T) {
	r := newTestRegistry()

	assert.Equal(t, "ecr", r.Registry("ecr").Name())
	assert.True(t, r.Registry("ecr").ShortLived())

	assert.Equal(t, "dockerhub", r.Registry("dockerhub").Name())
	assert.False(t, r.Registry("dockerhub").ShortLived())
	assert.Equal(t, "ghcr", r.Registry("ghcr").Name())

	// Unknown and empty provider keys fall back to the basic-auth "custom"
	// provider, preserving the legacy non-ECR behavior.
	assert.Equal(t, "custom", r.Registry("custom").Name())
	assert.Equal(t, "custom", r.Registry("").Name())
	assert.Equal(t, "custom", r.Registry("something-new").Name())
}

func TestRegistryObjectStorageLookup(t *testing.T) {
	r := newTestRegistry()

	assert.Equal(t, "aws_s3", r.ObjectStorage("aws_s3").Name())
	assert.Equal(t, "aws_s3", r.ObjectStorage("minio").Name())
	assert.Equal(t, "aws_s3", r.ObjectStorage("").Name())
}
