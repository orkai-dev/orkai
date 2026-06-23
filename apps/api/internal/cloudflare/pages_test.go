package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orkai-dev/orkai/apps/api/internal/cloudflare"
)

func writePagesResult(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	payload := map[string]any{"success": true, "errors": []any{}, "result": result}
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func TestPagesDefaultURL(t *testing.T) {
	assert.Equal(t, "https://my-site.pages.dev", cloudflare.PagesDefaultURL("my-site"))
}

func TestPagesAssetHash(t *testing.T) {
	hash := cloudflare.PagesAssetHash([]byte("hello"))
	assert.Len(t, hash, 64)
}

func TestAddPagesDomain(t *testing.T) {
	creds := cloudflare.Credentials{AuthMode: cloudflare.AuthAPIToken, APIToken: "token", AccountID: "acct-1"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/accounts/acct-1/pages/projects/orkai-site/domains", r.URL.Path)
		writePagesResult(t, w, cloudflare.PagesDomain{ID: "dom-1", Name: "app.example.com"})
	}))
	defer server.Close()

	c := cloudflare.NewTestClient(server.Client(), server.URL)
	out, err := c.AddPagesDomain(context.Background(), creds, "orkai-site", "app.example.com")
	require.NoError(t, err)
	assert.Equal(t, "app.example.com", out.Name)
}
