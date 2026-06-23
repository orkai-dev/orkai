package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// basicRegistry implements RegistryProvider for any registry that authenticates
// with a static username/password (Docker Hub, GHCR, or a custom registry). The
// defaultHost is used as the dockerconfigjson auths key when no URL is stored.
type basicRegistry struct {
	name        string
	defaultHost string
	client      *http.Client
}

func newBasicRegistry(name, defaultHost string, client *http.Client) *basicRegistry {
	return &basicRegistry{name: name, defaultHost: defaultHost, client: client}
}

func (p *basicRegistry) Name() string { return p.name }

func (p *basicRegistry) ShortLived() bool { return false }

type basicRegistryConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (p *basicRegistry) DockerAuth(ctx context.Context, cfg json.RawMessage) (host, username, password string, err error) {
	var c basicRegistryConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return "", "", "", fmt.Errorf("invalid registry config: %w", err)
	}
	return p.hostFor(c.URL), c.Username, c.Password, nil
}

// hostFor maps the configured URL (and provider default) to the registry host
// used as the dockerconfigjson auths key.
func (p *basicRegistry) hostFor(rawURL string) string {
	if rawURL == "" && p.defaultHost != "" {
		return p.defaultHost
	}
	return stripScheme(rawURL)
}

func (p *basicRegistry) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	var c basicRegistryConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return false, "invalid config", nil
	}

	registryURL := c.URL
	if registryURL == "" {
		registryURL = "https://registry-1.docker.io"
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", registryURL+"/v2/", nil)
	if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false, "connection failed: " + err.Error(), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 200 || resp.StatusCode == 401 {
		// 401 is expected for Docker Hub without a specific scope.
		return true, "registry reachable", nil
	}
	return false, fmt.Sprintf("registry error (HTTP %d)", resp.StatusCode), nil
}

// stripScheme removes a leading http(s):// scheme and trailing slash from a URL,
// leaving a bare host suitable as a dockerconfigjson auths key.
func stripScheme(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimSuffix(u, "/")
}

func init() {
	registerRegistry(func(d factoryDeps) RegistryProvider {
		return newBasicRegistry("dockerhub", "https://index.docker.io/v1/", d.client)
	})
	registerRegistry(func(d factoryDeps) RegistryProvider {
		return newBasicRegistry("ghcr", "ghcr.io", d.client)
	})
	registerRegistry(func(d factoryDeps) RegistryProvider {
		return newBasicRegistry("custom", "", d.client)
	})
}
