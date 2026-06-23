package providers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGitHub(settings fakeSettings) *githubProvider {
	return newGitHubProvider(settings, &http.Client{}, slog.Default())
}

func TestExtractGitHubOwner(t *testing.T) {
	assert.Equal(t, "owner", extractGitHubOwner("https://github.com/owner/repo.git"))
	assert.Equal(t, "owner", extractGitHubOwner("https://github.com/owner/repo"))
	assert.Equal(t, "", extractGitHubOwner("https://gitlab.com/owner/repo"))
}

func TestGenerateAppJWTInvalidPEM(t *testing.T) {
	_, err := generateAppJWT(123, "not-a-pem")
	require.ErrorContains(t, err, "decode PEM")
}

func TestGenerateAppJWTValid(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	token, err := generateAppJWT(123, string(pemBytes))
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestGenerateAppJWTWrongKeyType(t *testing.T) {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("garbage")})
	_, err := generateAppJWT(123, string(pemBytes))
	require.ErrorContains(t, err, "parse private key")
}

func TestInstallationTokenNoAppID(t *testing.T) {
	p := newTestGitHub(fakeSettings{})
	_, err := p.installationToken(context.Background(), "https://github.com/o/r")
	require.ErrorContains(t, err, "github_app_id not configured")
}

func TestInstallationTokenInvalidAppID(t *testing.T) {
	p := newTestGitHub(fakeSettings{"github_app_id": "not-a-number"})
	_, err := p.installationToken(context.Background(), "https://github.com/o/r")
	require.ErrorContains(t, err, "invalid github_app_id")
}

func TestInstallationTokenNoPEM(t *testing.T) {
	p := newTestGitHub(fakeSettings{"github_app_id": "123"})
	_, err := p.installationToken(context.Background(), "https://github.com/o/r")
	require.ErrorContains(t, err, "github_app_pem not configured")
}

func TestInstallationTokenNoOwner(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	p := newTestGitHub(fakeSettings{
		"github_app_id":  "123",
		"github_app_pem": string(pemBytes),
	})
	_, err := p.installationToken(context.Background(), "https://example.com/foo")
	require.ErrorContains(t, err, "cannot extract owner")
}

func TestGitHubCloneTokenFallsBackToStored(t *testing.T) {
	// No GitHub App configured → CloneToken falls back to the stored token.
	p := newTestGitHub(fakeSettings{})
	token, err := p.CloneToken(context.Background(), "https://github.com/o/r", json.RawMessage(`{"token":"ghp_stored"}`))
	require.NoError(t, err)
	assert.Equal(t, "ghp_stored", token)
}

func TestGitHubListReposInvalidConfig(t *testing.T) {
	p := newTestGitHub(fakeSettings{})
	_, err := p.ListRepos(context.Background(), json.RawMessage(`bad`))
	require.ErrorContains(t, err, "invalid config")
}

func TestGitHubRefreshNoOpWhenNoRefreshToken(t *testing.T) {
	p := newTestGitHub(fakeSettings{})
	cfg := json.RawMessage(`{"token":"t"}`)
	updated, changed, err := p.Refresh(context.Background(), cfg)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, cfg, updated)
}

func TestGitHubRefreshNoOpOnInvalidConfig(t *testing.T) {
	p := newTestGitHub(fakeSettings{})
	cfg := json.RawMessage(`bad`)
	updated, changed, err := p.Refresh(context.Background(), cfg)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, cfg, updated)
}

func TestGitHubVerifyWebhook(t *testing.T) {
	p := newTestGitHub(fakeSettings{})
	secret := "shhh"
	body := []byte(`{"ref":"refs/heads/main"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	good := http.Header{}
	good.Set("X-Hub-Signature-256", sig)
	assert.True(t, p.VerifyWebhook(secret, body, good))

	bad := http.Header{}
	bad.Set("X-Hub-Signature-256", "sha256=deadbeef")
	assert.False(t, p.VerifyWebhook(secret, body, bad))
}
