package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripScheme(t *testing.T) {
	assert.Equal(t, "example.com", stripScheme("https://example.com/"))
	assert.Equal(t, "example.com", stripScheme("http://example.com"))
	assert.Equal(t, "example.com", stripScheme("example.com"))
}

func TestBasicRegistryHostDerivation(t *testing.T) {
	client := &http.Client{}
	dockerhub := newBasicRegistry("dockerhub", "https://index.docker.io/v1/", client)
	ghcr := newBasicRegistry("ghcr", "ghcr.io", client)
	custom := newBasicRegistry("custom", "", client)

	assert.Equal(t, "https://index.docker.io/v1/", dockerhub.hostFor(""))
	assert.Equal(t, "ghcr.io", ghcr.hostFor(""))
	// A configured URL always wins over the default and is reduced to a bare host.
	assert.Equal(t, "reg.example.com", dockerhub.hostFor("https://reg.example.com"))
	assert.Equal(t, "my.registry.io", custom.hostFor("https://my.registry.io/"))
}

func TestBasicRegistryDockerAuth(t *testing.T) {
	dockerhub := newBasicRegistry("dockerhub", "https://index.docker.io/v1/", &http.Client{})
	host, user, pass, err := dockerhub.DockerAuth(context.Background(), json.RawMessage(`{"username":"alice","password":"secret"}`))
	require.NoError(t, err)
	assert.Equal(t, "https://index.docker.io/v1/", host)
	assert.Equal(t, "alice", user)
	assert.Equal(t, "secret", pass)
}

func TestBasicRegistryDockerAuthInvalidConfig(t *testing.T) {
	custom := newBasicRegistry("custom", "", &http.Client{})
	_, _, _, err := custom.DockerAuth(context.Background(), json.RawMessage(`bad`))
	require.ErrorContains(t, err, "invalid registry config")
}

func TestBasicRegistryTestConnectionInvalidConfig(t *testing.T) {
	custom := newBasicRegistry("custom", "", &http.Client{})
	ok, msg, err := custom.TestConnection(context.Background(), json.RawMessage(`bad`))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "invalid config", msg)
}
