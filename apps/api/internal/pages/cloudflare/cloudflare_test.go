package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cf "github.com/orkai-dev/orkai/apps/api/internal/cloudflare"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	pagescf "github.com/orkai-dev/orkai/apps/api/internal/pages/cloudflare"
)

func testCreds() json.RawMessage {
	cfg, _ := json.Marshal(cf.Credentials{
		AuthMode:  cf.AuthAPIToken,
		APIToken:  "token",
		AccountID: "acct-1",
	})
	return cfg
}

func writeResult(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	payload := map[string]any{"success": true, "errors": []any{}, "result": result}
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func TestPagesClient_CreateProjectAndDeployment(t *testing.T) {
	creds := cf.Credentials{AuthMode: cf.AuthAPIToken, APIToken: "token", AccountID: "acct-1"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/accounts/acct-1/pages/projects":
			writeResult(t, w, cf.PagesProject{ID: "proj-1", Name: "orkai-site", Subdomain: "orkai-site"})
		case r.Method == http.MethodGet && r.URL.Path == "/accounts/acct-1/pages/projects/orkai-site/upload-token":
			writeResult(t, w, map[string]string{"jwt": "jwt-token"})
		case r.Method == http.MethodPost && r.URL.Path == "/pages/assets/upload":
			writeResult(t, w, map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/accounts/acct-1/pages/projects/orkai-site/deployments":
			var body struct {
				Manifest map[string]string `json:"manifest"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Contains(t, body.Manifest, "index.html")
			writeResult(t, w, cf.PagesDeployment{ID: "dep-1"})
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := cf.NewTestClient(server.Client(), server.URL)
	proj, err := c.CreatePagesProject(context.Background(), creds, "orkai-site")
	require.NoError(t, err)
	assert.Equal(t, "proj-1", proj.ID)

	jwt, err := c.PagesUploadToken(context.Background(), creds, "orkai-site")
	require.NoError(t, err)
	assert.Equal(t, "jwt-token", jwt)

	content := []byte("<html></html>")
	hash := cf.PagesAssetHash(content)
	require.NoError(t, c.UploadPagesAsset(context.Background(), jwt, hash, content))

	dep, err := c.CreatePagesDeployment(context.Background(), creds, "orkai-site", map[string]string{
		"index.html": hash,
	})
	require.NoError(t, err)
	assert.Equal(t, "dep-1", dep.ID)
}

func TestPagesProvider_DeployDirectUpload(t *testing.T) {
	credsJSON := testCreds()
	uploaded := map[string]struct{}{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/accounts/acct-1/pages/projects/orkai-site/upload-token":
			writeResult(t, w, map[string]string{"jwt": "jwt-token"})
		case r.Method == http.MethodPost && r.URL.Path == "/pages/assets/upload":
			require.NoError(t, r.ParseMultipartForm(10<<20))
			for key := range r.MultipartForm.File {
				uploaded[key] = struct{}{}
			}
			writeResult(t, w, map[string]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/accounts/acct-1/pages/projects/orkai-site/deployments":
			writeResult(t, w, cf.PagesDeployment{ID: "dep-99"})
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	filesDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(filesDir, "index.html"), []byte("hello"), 0o644))

	p := pagescf.NewWithClient(cf.NewTestClient(server.Client(), server.URL))
	page := &model.Page{
		BaseModel: model.BaseModel{ID: uuid.New()},
		Name:      "site",
		Runtime: &model.PageRuntime{
			CFProjectID:   "proj-1",
			CFProjectName: "orkai-site",
			DefaultURL:    "https://orkai-site.pages.dev",
		},
	}

	result, err := p.Deploy(context.Background(), page, credsJSON, filesDir, func(string) {})
	require.NoError(t, err)
	assert.Equal(t, "dep-99", result.ProviderRef)
	assert.Len(t, uploaded, 1)
}

func TestPagesProvider_ProvisionCreatesProject(t *testing.T) {
	credsJSON := testCreds()
	var saved *model.PageRuntime

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/accounts/acct-1/pages/projects" {
			writeResult(t, w, cf.PagesProject{
				ID:        "proj-1",
				Name:      "orkai-marketing",
				Subdomain: "orkai-marketing",
			})
			return
		}
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
	}))
	defer server.Close()

	p := pagescf.NewWithClient(cf.NewTestClient(server.Client(), server.URL))
	page := &model.Page{
		BaseModel: model.BaseModel{ID: uuid.New()},
		Name:      "marketing",
		Runtime:   &model.PageRuntime{},
	}

	rt, err := p.Provision(context.Background(), page, credsJSON, nil, func(_ context.Context, next *model.PageRuntime) error {
		saved = next
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "proj-1", rt.CFProjectID)
	assert.Equal(t, "https://orkai-marketing.pages.dev", rt.DefaultURL)
	require.NotNil(t, saved)
	assert.Equal(t, "proj-1", saved.CFProjectID)
}
