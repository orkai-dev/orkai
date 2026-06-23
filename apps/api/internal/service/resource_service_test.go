package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newResourceService(fs *testsupport.FakeStore) *ResourceService {
	return NewResourceService(fs, testsupport.NewProviders(fs), testsupport.NewTestLogger(), nil)
}

func TestResourceCreateAutoName(t *testing.T) {
	fs := testsupport.NewFakeStore()
	var saved *model.SharedResource
	fs.SharedResourcesStore.CreateFn = func(ctx context.Context, r *model.SharedResource) error {
		saved = r
		return nil
	}
	s := newResourceService(fs)
	res, err := s.Create(context.Background(), uuid.New(), CreateResourceInput{Type: model.ResourceSSHKey, Provider: "ed25519"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Name)
	assert.Equal(t, "active", saved.Status)
}

func TestResourceCreateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.CreateFn = func(ctx context.Context, r *model.SharedResource) error {
		return errors.New("insert failed")
	}
	s := newResourceService(fs)
	_, err := s.Create(context.Background(), uuid.New(), CreateResourceInput{Name: "x", Type: model.ResourceRegistry})
	require.Error(t, err)
}

func TestAutoNameVariants(t *testing.T) {
	s := newResourceService(testsupport.NewFakeStore())

	git := s.autoName(CreateResourceInput{Type: model.ResourceGitProvider, Provider: "github", Config: json.RawMessage(`{"username":"alan"}`)})
	assert.Contains(t, git, "github-alan-")

	gitNoUser := s.autoName(CreateResourceInput{Type: model.ResourceGitProvider, Provider: "github"})
	assert.Contains(t, gitNoUser, "github-git-")

	reg := s.autoName(CreateResourceInput{Type: model.ResourceRegistry, Provider: "dockerhub", Config: json.RawMessage(`{"username":"me"}`)})
	assert.Contains(t, reg, "dockerhub-me-")

	regNoUser := s.autoName(CreateResourceInput{Type: model.ResourceRegistry, Provider: "dockerhub"})
	assert.Contains(t, regNoUser, "dockerhub-registry-")

	ssh := s.autoName(CreateResourceInput{Type: model.ResourceSSHKey, Provider: "ed25519"})
	assert.Contains(t, ssh, "key-ed25519-")

	store := s.autoName(CreateResourceInput{Type: model.ResourceObjectStorage, Provider: "r2", Config: json.RawMessage(`{"bucket":"data"}`)})
	assert.Contains(t, store, "r2-data-")

	storeNoBucket := s.autoName(CreateResourceInput{Type: model.ResourceObjectStorage, Provider: "r2"})
	assert.Contains(t, storeNoBucket, "r2-storage-")

	def := s.autoName(CreateResourceInput{Type: model.ResourceType("custom")})
	assert.Contains(t, def, "resource-")
}

func TestGenerateSSHKeyEd25519(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newResourceService(fs)
	res, err := s.GenerateSSHKey(context.Background(), uuid.New(), "ed25519", "mykey")
	require.NoError(t, err)
	assert.Equal(t, "mykey", res.Name)

	var cfg map[string]string
	require.NoError(t, json.Unmarshal(res.Config, &cfg))
	assert.Contains(t, cfg["public_key"], "ssh-ed25519")
	assert.NotEmpty(t, cfg["private_key"])
}

func TestGenerateSSHKeyRSAAutoName(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newResourceService(fs)
	res, err := s.GenerateSSHKey(context.Background(), uuid.New(), "rsa-4096", "")
	require.NoError(t, err)
	assert.Contains(t, res.Name, "key-")
}

func TestGenerateSSHKeyUnsupported(t *testing.T) {
	s := newResourceService(testsupport.NewFakeStore())
	_, err := s.GenerateSSHKey(context.Background(), uuid.New(), "dsa", "k")
	require.ErrorContains(t, err, "unsupported algorithm")
}

func TestGenerateSSHKeyStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.CreateFn = func(ctx context.Context, r *model.SharedResource) error {
		return errors.New("insert failed")
	}
	s := newResourceService(fs)
	_, err := s.GenerateSSHKey(context.Background(), uuid.New(), "ed25519", "k")
	require.Error(t, err)
}

func TestResourceGetByIDAndList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	rid := uuid.New()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}}, nil
	}
	fs.SharedResourcesStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID, rt string) ([]model.SharedResource, error) {
		return []model.SharedResource{{}}, nil
	}
	s := newResourceService(fs)

	got, err := s.GetByID(context.Background(), rid)
	require.NoError(t, err)
	assert.Equal(t, rid, got.ID)

	list, err := s.List(context.Background(), uuid.New(), "")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestResourceUpdate(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}, OrgID: orgID}, nil
	}
	s := newResourceService(fs)
	name, provider := "new", "ghcr"
	cfg := json.RawMessage(`{"a":"b"}`)
	res, err := s.Update(context.Background(), orgID, rid, UpdateResourceInput{Name: &name, Provider: &provider, Config: &cfg})
	require.NoError(t, err)
	assert.Equal(t, "new", res.Name)
	assert.Equal(t, "ghcr", res.Provider)
}

func TestResourceUpdateNotOwned(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: uuid.New()}, nil
	}
	s := newResourceService(fs)
	_, err := s.Update(context.Background(), uuid.New(), uuid.New(), UpdateResourceInput{})
	require.Error(t, err)
}

func TestResourceUpdateGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return nil, errors.New("missing")
	}
	s := newResourceService(fs)
	_, err := s.Update(context.Background(), uuid.New(), uuid.New(), UpdateResourceInput{})
	require.Error(t, err)
}

func TestResourceUpdateStoreError(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID}, nil
	}
	fs.SharedResourcesStore.UpdateFn = func(ctx context.Context, r *model.SharedResource) error {
		return errors.New("update failed")
	}
	s := newResourceService(fs)
	_, err := s.Update(context.Background(), orgID, uuid.New(), UpdateResourceInput{})
	require.Error(t, err)
}

func TestResourceDeleteSuccess(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID}, nil
	}
	deleted := false
	fs.SharedResourcesStore.DeleteFn = func(ctx context.Context, id uuid.UUID) error {
		deleted = true
		return nil
	}
	s := newResourceService(fs)
	require.NoError(t, s.Delete(context.Background(), orgID, uuid.New()))
	assert.True(t, deleted)
}

func TestResourceDeleteInUseByApp(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}, OrgID: orgID}, nil
	}
	fs.ApplicationsStore.FindByResourceFn = func(ctx context.Context, resourceID uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web", GitProviderID: &rid}, nil
	}
	s := newResourceService(fs)
	err := s.Delete(context.Background(), orgID, rid)
	require.ErrorContains(t, err, "git provider")
}

func TestResourceDeleteInUseByRegistry(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}, OrgID: orgID}, nil
	}
	fs.ApplicationsStore.FindByResourceFn = func(ctx context.Context, resourceID uuid.UUID) (*model.Application, error) {
		return &model.Application{Name: "web", RegistryID: &rid}, nil
	}
	s := newResourceService(fs)
	err := s.Delete(context.Background(), orgID, rid)
	require.ErrorContains(t, err, "registry")
}

func TestResourceDeleteAppListError(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID}, nil
	}
	fs.ApplicationsStore.FindByResourceFn = func(ctx context.Context, resourceID uuid.UUID) (*model.Application, error) {
		return nil, errors.New("boom")
	}
	s := newResourceService(fs)
	require.Error(t, s.Delete(context.Background(), orgID, uuid.New()))
}

func TestResourceDeleteInUseByNode(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}, OrgID: orgID}, nil
	}
	fs.ServerNodesStore.FindBySSHKeyFn = func(ctx context.Context, sshKeyID uuid.UUID) (*model.ServerNode, error) {
		return &model.ServerNode{Name: "node1", SSHKeyID: &rid}, nil
	}
	s := newResourceService(fs)
	require.ErrorContains(t, s.Delete(context.Background(), orgID, rid), "SSH key")
}

func TestResourceDeleteNodeListError(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID}, nil
	}
	fs.ServerNodesStore.FindBySSHKeyFn = func(ctx context.Context, sshKeyID uuid.UUID) (*model.ServerNode, error) {
		return nil, errors.New("boom")
	}
	s := newResourceService(fs)
	require.Error(t, s.Delete(context.Background(), orgID, uuid.New()))
}

func TestResourceDeleteInUseByDatabase(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}, OrgID: orgID}, nil
	}
	fs.ManagedDatabasesStore.FindByBackupS3Fn = func(ctx context.Context, s3ID uuid.UUID) (*model.ManagedDatabase, error) {
		return &model.ManagedDatabase{Name: "db1", BackupS3ID: &rid}, nil
	}
	s := newResourceService(fs)
	require.ErrorContains(t, s.Delete(context.Background(), orgID, rid), "backup storage")
}

func TestResourceDeleteDatabaseLookupError(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID}, nil
	}
	fs.ManagedDatabasesStore.FindByBackupS3Fn = func(ctx context.Context, s3ID uuid.UUID) (*model.ManagedDatabase, error) {
		return nil, errors.New("boom")
	}
	s := newResourceService(fs)
	require.Error(t, s.Delete(context.Background(), orgID, uuid.New()))
}

func TestResourceDeleteInUseByPage(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}, OrgID: orgID}, nil
	}
	fs.PagesStore.FindByResourceFn = func(ctx context.Context, resourceID uuid.UUID) (*model.Page, error) {
		return &model.Page{Name: "site", CloudAccountID: &rid}, nil
	}
	s := newResourceService(fs)
	require.ErrorContains(t, s.Delete(context.Background(), orgID, rid), "cloud account")
}

func TestResourceDeleteInUseBySystemBackup(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{BaseModel: model.BaseModel{ID: rid}, OrgID: orgID}, nil
	}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return rid.String(), nil
	}
	s := newResourceService(fs)
	require.ErrorContains(t, s.Delete(context.Background(), orgID, rid), "system backup storage")
}

func TestTestConnectionSSHKey(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceSSHKey}, nil
	}
	s := newResourceService(fs)
	ok, msg, err := s.TestConnection(context.Background(), orgID, uuid.New())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, msg, "SSH key")
}

func TestTestConnectionUnknownType(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceType("weird")}, nil
	}
	s := newResourceService(fs)
	ok, _, err := s.TestConnection(context.Background(), orgID, uuid.New())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTestConnectionNotOwned(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: uuid.New()}, nil
	}
	s := newResourceService(fs)
	_, _, err := s.TestConnection(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

func TestTestConnectionObjectStorageValidation(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	// invalid JSON config
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceObjectStorage, Config: json.RawMessage(`not-json`)}, nil
	}
	s := newResourceService(fs)
	ok, msg, err := s.TestConnection(context.Background(), orgID, uuid.New())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "invalid config", msg)
}

func TestTestConnectionObjectStorageMissingFields(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceObjectStorage, Config: json.RawMessage(`{"bucket":"b"}`)}, nil
	}
	s := newResourceService(fs)
	ok, msg, err := s.TestConnection(context.Background(), orgID, uuid.New())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, msg, "endpoint is required")
}

func TestTestConnectionGitProviderInvalidConfig(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceGitProvider, Provider: "github", Config: json.RawMessage(`bad`)}, nil
	}
	s := newResourceService(fs)
	ok, msg, _ := s.TestConnection(context.Background(), orgID, uuid.New())
	assert.False(t, ok)
	assert.Equal(t, "invalid config", msg)
}

func TestTestConnectionRegistryInvalidConfig(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceRegistry, Config: json.RawMessage(`bad`)}, nil
	}
	s := newResourceService(fs)
	ok, msg, _ := s.TestConnection(context.Background(), orgID, uuid.New())
	assert.False(t, ok)
	assert.Equal(t, "invalid config", msg)
}

func TestListReposNotGitProvider(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceSSHKey}, nil
	}
	s := newResourceService(fs)
	_, err := s.ListRepos(context.Background(), orgID, uuid.New())
	require.ErrorContains(t, err, "not a git provider")
}

func TestListReposNotOwned(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: uuid.New()}, nil
	}
	s := newResourceService(fs)
	_, err := s.ListRepos(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

func TestListReposUnsupportedProvider(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceGitProvider, Provider: "bitbucket", Config: json.RawMessage(`{"token":"t"}`)}, nil
	}
	s := newResourceService(fs)
	_, err := s.ListRepos(context.Background(), orgID, uuid.New())
	require.ErrorContains(t, err, "unsupported provider")
}

func TestListReposGiteaNoURL(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceGitProvider, Provider: "gitea", Config: json.RawMessage(`{"token":"t"}`)}, nil
	}
	s := newResourceService(fs)
	_, err := s.ListRepos(context.Background(), orgID, uuid.New())
	require.ErrorContains(t, err, "gitea API URL not configured")
}

func TestListReposInvalidConfig(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceGitProvider, Provider: "github", Config: json.RawMessage(`bad`)}, nil
	}
	s := newResourceService(fs)
	_, err := s.ListRepos(context.Background(), orgID, uuid.New())
	require.ErrorContains(t, err, "invalid config")
}

func TestRedactResourceConfig(t *testing.T) {
	r := model.SharedResource{
		Type:   model.ResourceGitProvider,
		Config: json.RawMessage(`{"token":"ghp_secret","username":"alan","private_key":"PRIV","public_key":"PUB"}`),
	}
	redacted := RedactResourceConfig(r)

	var cfg map[string]string
	require.NoError(t, json.Unmarshal(redacted.Config, &cfg))
	assert.Equal(t, model.SettingSecretMask, cfg["token"])
	assert.Equal(t, model.SettingSecretMask, cfg["private_key"])
	assert.Equal(t, "alan", cfg["username"])
	assert.Equal(t, "PUB", cfg["public_key"])

	// Original resource config must be untouched.
	assert.Contains(t, string(r.Config), "ghp_secret")
}

func TestRedactResourceConfigEmptySecretNotMasked(t *testing.T) {
	r := model.SharedResource{Config: json.RawMessage(`{"token":""}`)}
	redacted := RedactResourceConfig(r)
	assert.JSONEq(t, `{"token":""}`, string(redacted.Config))
}

func TestResourceUpdatePreservesMaskedSecret(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{
			BaseModel: model.BaseModel{ID: rid},
			OrgID:     orgID,
			Config:    json.RawMessage(`{"token":"real-token","username":"alan"}`),
		}, nil
	}
	var saved *model.SharedResource
	fs.SharedResourcesStore.UpdateFn = func(ctx context.Context, r *model.SharedResource) error {
		saved = r
		return nil
	}
	s := newResourceService(fs)
	// Client re-submits the masked token (unchanged) but edits the username.
	cfg := json.RawMessage(`{"token":"` + model.SettingSecretMask + `","username":"bob"}`)
	_, err := s.Update(context.Background(), orgID, rid, UpdateResourceInput{Config: &cfg})
	require.NoError(t, err)

	var out map[string]string
	require.NoError(t, json.Unmarshal(saved.Config, &out))
	assert.Equal(t, "real-token", out["token"])
	assert.Equal(t, "bob", out["username"])
}

func TestResourceUpdateMaskedSecretAbsentInStoreIsDropped(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	// Stored config has no "token" key at all.
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{
			BaseModel: model.BaseModel{ID: rid},
			OrgID:     orgID,
			Config:    json.RawMessage(`{"username":"alan"}`),
		}, nil
	}
	var saved *model.SharedResource
	fs.SharedResourcesStore.UpdateFn = func(ctx context.Context, r *model.SharedResource) error {
		saved = r
		return nil
	}
	s := newResourceService(fs)
	// Client sends the masked token, but it does not exist in the stored config.
	cfg := json.RawMessage(`{"token":"` + model.SettingSecretMask + `","username":"alan"}`)
	_, err := s.Update(context.Background(), orgID, rid, UpdateResourceInput{Config: &cfg})
	require.NoError(t, err)

	// Must not persist a JSON null credential.
	assert.NotContains(t, string(saved.Config), "null")
	var out map[string]any
	require.NoError(t, json.Unmarshal(saved.Config, &out))
	_, hasToken := out["token"]
	assert.False(t, hasToken, "absent masked secret should be dropped, not nulled")
	assert.Equal(t, "alan", out["username"])
}

func TestResourceUpdateReplacesSecretWhenChanged(t *testing.T) {
	orgID := uuid.New()
	rid := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{
			BaseModel: model.BaseModel{ID: rid},
			OrgID:     orgID,
			Config:    json.RawMessage(`{"token":"old-token"}`),
		}, nil
	}
	var saved *model.SharedResource
	fs.SharedResourcesStore.UpdateFn = func(ctx context.Context, r *model.SharedResource) error {
		saved = r
		return nil
	}
	s := newResourceService(fs)
	cfg := json.RawMessage(`{"token":"new-token"}`)
	_, err := s.Update(context.Background(), orgID, rid, UpdateResourceInput{Config: &cfg})
	require.NoError(t, err)

	var out map[string]string
	require.NoError(t, json.Unmarshal(saved.Config, &out))
	assert.Equal(t, "new-token", out["token"])
}

func TestShortHex(t *testing.T) {
	assert.Len(t, shortHex(4), 8)
}

func TestDNSZonesNotCloudAccount(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceObjectStorage}, nil
	}
	s := newResourceService(fs)
	_, err := s.DNSZones(context.Background(), orgID, uuid.New())
	require.ErrorContains(t, err, "not a cloud account")
}

func TestDNSZonesNotOwned(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: uuid.New(), Type: model.ResourceCloudAccount}, nil
	}
	s := newResourceService(fs)
	_, err := s.DNSZones(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
}

func TestDNSZonesInvalidConfig(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{
			OrgID:  orgID,
			Type:   model.ResourceCloudAccount,
			Config: json.RawMessage(`bad`),
		}, nil
	}
	s := newResourceService(fs)
	_, err := s.DNSZones(context.Background(), orgID, uuid.New())
	require.ErrorContains(t, err, "invalid config")
}

func TestDNSRecordsNotCloudAccount(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: orgID, Type: model.ResourceGitProvider}, nil
	}
	s := newResourceService(fs)
	_, err := s.DNSRecords(context.Background(), orgID, uuid.New(), "Z123")
	require.ErrorContains(t, err, "not a cloud account")
}

func TestDNSUpsertRecordInvalidConfig(t *testing.T) {
	orgID := uuid.New()
	fs := testsupport.NewFakeStore()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{
			OrgID:  orgID,
			Type:   model.ResourceCloudAccount,
			Config: json.RawMessage(`not-json`),
		}, nil
	}
	s := newResourceService(fs)
	err := s.DNSUpsertRecord(context.Background(), orgID, uuid.New(), DNSUpsertRecordInput{
		ZoneID: "Z123",
		Name:   "app.example.com",
		Type:   "A",
		Values: []string{"1.2.3.4"},
	})
	require.ErrorContains(t, err, "invalid config")
}
