package v1

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

func newGitHubOAuthHandler(fs *testsupport.FakeStore) *GitHubOAuthHandler {
	resSvc := service.NewResourceService(fs, testsupport.NewProviders(fs), testLogger(), nil)
	return NewGitHubOAuthHandler(fs, resSvc, "http://localhost:3000", testLogger())
}

func TestGitHubSetupManifest(t *testing.T) {
	h := newGitHubOAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/auth/github/setup", h.SetupManifest)
	assert.Equal(t, 200, doJSON(r, "GET", "/auth/github/setup?org=acme", nil).Code)
	assert.Equal(t, 200, doJSON(r, "GET", "/auth/github/setup", nil).Code)
}

func TestGitHubStatus(t *testing.T) {
	h := newGitHubOAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/auth/github/status", h.GitHubStatus)
	assert.Equal(t, 200, doJSON(r, "GET", "/auth/github/status", nil).Code)
}

func TestGitHubConnectNotConfigured(t *testing.T) {
	h := newGitHubOAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/auth/github/connect", h.Connect)
	assert.Equal(t, 400, doJSON(r, "GET", "/auth/github/connect", nil).Code)
}

func TestGitHubConnectConfigured(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == "github_app_client_id" {
			return "cid", nil
		}
		return "", nil
	}
	h := newGitHubOAuthHandler(fs)
	r, _, _ := newAuthedRouter()
	r.GET("/auth/github/connect", h.Connect)
	assert.Equal(t, 200, doJSON(r, "GET", "/auth/github/connect?type=org&org=acme", nil).Code)
}

func TestGitHubSetupCallbackMissingCode(t *testing.T) {
	h := newGitHubOAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/auth/github/setup/callback", h.SetupCallback)
	assert.Equal(t, 302, doJSON(r, "GET", "/auth/github/setup/callback", nil).Code)
}

func TestGitHubCallbackMissingCode(t *testing.T) {
	h := newGitHubOAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/auth/github/callback", h.Callback)
	assert.Equal(t, 302, doJSON(r, "GET", "/auth/github/callback", nil).Code)
}

func TestGitHubCallbackInvalidState(t *testing.T) {
	h := newGitHubOAuthHandler(testsupport.NewFakeStore())
	r, _, _ := newAuthedRouter()
	r.GET("/auth/github/callback", h.Callback)
	assert.Equal(t, 302, doJSON(r, "GET", "/auth/github/callback?code=x&state=y", nil).Code)
}

func TestGitHubResolveOrgID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	oid := uuid.New()
	uid := uuid.New()
	fs.UsersStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{BaseModel: model.BaseModel{ID: id}, OrgID: oid}, nil
	}
	h := newGitHubOAuthHandler(fs)
	got, err := h.resolveOrgID(context.Background(), uid.String())
	assert.NoError(t, err)
	assert.Equal(t, oid, got)

	_, err = h.resolveOrgID(context.Background(), "not-a-uuid")
	assert.Error(t, err)
}
