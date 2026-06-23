package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitLabVerifyWebhook(t *testing.T) {
	p := newGitLabProvider(&http.Client{})
	body := []byte(`{}`)

	good := http.Header{}
	good.Set("X-Gitlab-Token", "secret")
	assert.True(t, p.VerifyWebhook("secret", body, good))

	bad := http.Header{}
	bad.Set("X-Gitlab-Token", "wrong")
	assert.False(t, p.VerifyWebhook("secret", body, bad))
}

func TestGitLabCloneTokenUsesStored(t *testing.T) {
	p := newGitLabProvider(&http.Client{})
	token, err := p.CloneToken(context.Background(), "https://gitlab.com/o/r", json.RawMessage(`{"token":"glpat"}`))
	require.NoError(t, err)
	assert.Equal(t, "glpat", token)
}

func TestGitLabCloneTokenMissing(t *testing.T) {
	p := newGitLabProvider(&http.Client{})
	_, err := p.CloneToken(context.Background(), "https://gitlab.com/o/r", json.RawMessage(`{}`))
	require.ErrorContains(t, err, "no token available")
}

func TestGiteaListReposNoURL(t *testing.T) {
	p := newGiteaProvider(&http.Client{})
	_, err := p.ListRepos(context.Background(), json.RawMessage(`{"token":"t"}`))
	require.ErrorContains(t, err, "gitea API URL not configured")
}

func TestGiteaTestConnectionNoURL(t *testing.T) {
	p := newGiteaProvider(&http.Client{})
	ok, msg, err := p.TestConnection(context.Background(), json.RawMessage(`{"token":"t"}`))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "gitea API URL not configured", msg)
}

func TestGiteaListReposInvalidConfig(t *testing.T) {
	p := newGiteaProvider(&http.Client{})
	_, err := p.ListRepos(context.Background(), json.RawMessage(`bad`))
	require.ErrorContains(t, err, "invalid config")
}

func TestGiteaVerifyWebhookUnsupported(t *testing.T) {
	p := newGiteaProvider(&http.Client{})
	assert.False(t, p.VerifyWebhook("secret", []byte(`{}`), http.Header{}))
}
