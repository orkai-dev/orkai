package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRegistryAuth(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *RegistryAuth {
	return NewRegistryAuth(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewProviders(fs), testsupport.NewTestLogger())
}

func TestDockerConfigJSONForAppNoRegistry(t *testing.T) {
	r := newRegistryAuth(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	cfg, err := r.DockerConfigJSONForApp(context.Background(), &model.Application{})
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestDockerConfigJSONForAppWithRegistry(t *testing.T) {
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceRegistry, Provider: "dockerhub", Config: json.RawMessage(`{"username":"u","password":"p"}`)}, nil
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	cfg, err := r.DockerConfigJSONForApp(context.Background(), &model.Application{RegistryID: &rid})
	require.NoError(t, err)
	assert.Contains(t, string(cfg), "index.docker.io")
}

func TestDockerConfigJSONNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return nil, errors.New("missing")
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	_, err := r.DockerConfigJSON(context.Background(), uuid.New())
	require.ErrorContains(t, err, "registry not found")
}

func TestDockerConfigJSONWrongType(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceSSHKey}, nil
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	_, err := r.DockerConfigJSON(context.Background(), uuid.New())
	require.ErrorContains(t, err, "is not a registry")
}

func TestDockerConfigJSONECRInvalidConfig(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceRegistry, Provider: "ecr", Config: json.RawMessage(`bad`)}, nil
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	_, err := r.DockerConfigJSON(context.Background(), uuid.New())
	require.ErrorContains(t, err, "invalid ecr config")
}

func TestDockerConfigJSONBasicAuthInvalidConfig(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceRegistry, Provider: "custom", Config: json.RawMessage(`bad`)}, nil
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	_, err := r.DockerConfigJSON(context.Background(), uuid.New())
	require.ErrorContains(t, err, "invalid registry config")
}

func TestDockerConfigJSONDockerhubSuccess(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Type: model.ResourceRegistry, Provider: "dockerhub", Config: json.RawMessage(`{"username":"alice","password":"secret"}`)}, nil
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	cfg, err := r.DockerConfigJSON(context.Background(), uuid.New())
	require.NoError(t, err)

	var doc dockerConfigJSON
	require.NoError(t, json.Unmarshal(cfg, &doc))
	entry := doc.Auths["https://index.docker.io/v1/"]
	assert.Equal(t, "alice", entry.Username)
	decoded, _ := base64.StdEncoding.DecodeString(entry.Auth)
	assert.Equal(t, "alice:secret", string(decoded))
}

func TestBuildDockerConfigJSON(t *testing.T) {
	cfg, err := buildDockerConfigJSON("host.io", "u", "p")
	require.NoError(t, err)
	assert.Contains(t, string(cfg), "host.io")
}

func TestRefreshECRSecretsListError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.ListWithRegistryFn = func(ctx context.Context) ([]model.Application, error) {
		return nil, errors.New("db down")
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	r.refreshECRSecrets(context.Background()) // should not panic
}

func TestRefreshECRSecretsSkipsNonECR(t *testing.T) {
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.ApplicationsStore.ListWithRegistryFn = func(ctx context.Context) ([]model.Application, error) {
		return []model.Application{
			{Name: "no-reg"},                    // RegistryID nil → continue
			{Name: "non-ecr", RegistryID: &rid}, // resource provider not ecr → continue
		}, nil
	}
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{Provider: "dockerhub"}, nil
	}
	r := newRegistryAuth(fs, testsupport.NewFakeOrchestrator())
	r.refreshECRSecrets(context.Background())
}
