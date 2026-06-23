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

func TestResolveRegistryConfigNonECRPassthrough(t *testing.T) {
	fs := testsupport.NewFakeStore()
	cfg := json.RawMessage(`{"username":"u","password":"p"}`)
	out, err := resolveRegistryConfig(context.Background(), fs, uuid.New(), "dockerhub", cfg)
	require.NoError(t, err)
	assert.Equal(t, cfg, out)
}

func TestResolveRegistryConfigECRStaticKeysPassthrough(t *testing.T) {
	// An ECR config carrying its own static keys (no cloud_account_id) is left
	// untouched.
	fs := testsupport.NewFakeStore()
	cfg := json.RawMessage(`{"region":"us-east-1","access_key":"AKIA","secret_key":"sk"}`)
	out, err := resolveRegistryConfig(context.Background(), fs, uuid.New(), "ecr", cfg)
	require.NoError(t, err)
	assert.Equal(t, cfg, out)
}

func TestResolveRegistryConfigCloudAccountStaticKeys(t *testing.T) {
	orgID := uuid.New()
	accID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		assert.Equal(t, accID, id)
		return &model.SharedResource{
			OrgID:    orgID,
			Type:     model.ResourceCloudAccount,
			Provider: "aws",
			Config:   json.RawMessage(`{"auth_mode":"access_key","access_key_id":"AKIA","secret_access_key":"sek"}`),
		}, nil
	}
	cfg := json.RawMessage(`{"region":"us-east-1","cloud_account_id":"` + accID.String() + `"}`)
	out, err := resolveRegistryConfig(context.Background(), fs, orgID, "ecr", cfg)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	assert.Equal(t, "AKIA", m["access_key"])
	assert.Equal(t, "sek", m["secret_key"])
	assert.Equal(t, "us-east-1", m["region"])
}

func TestResolveRegistryConfigCloudAccountCrossOrgRejected(t *testing.T) {
	accID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{
			OrgID:    uuid.New(), // different org
			Type:     model.ResourceCloudAccount,
			Provider: "aws",
			Config:   json.RawMessage(`{}`),
		}, nil
	}
	cfg := json.RawMessage(`{"region":"us-east-1","cloud_account_id":"` + accID.String() + `"}`)
	_, err := resolveRegistryConfig(context.Background(), fs, uuid.New(), "ecr", cfg)
	require.ErrorContains(t, err, "cloud account not found")
}

func TestResolveRegistryConfigCloudAccountNonAWSRejected(t *testing.T) {
	orgID := uuid.New()
	accID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{
			OrgID:    orgID,
			Type:     model.ResourceCloudAccount,
			Provider: "cloudflare",
			Config:   json.RawMessage(`{}`),
		}, nil
	}
	cfg := json.RawMessage(`{"region":"us-east-1","cloud_account_id":"` + accID.String() + `"}`)
	_, err := resolveRegistryConfig(context.Background(), fs, orgID, "ecr", cfg)
	require.ErrorContains(t, err, "not an AWS account")
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
