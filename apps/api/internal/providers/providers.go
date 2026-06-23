// Package providers centralizes all per-integration (git host, container
// registry) behavior behind small capability interfaces. Adding a new
// integration means implementing an interface and registering it once in
// registry.go — there is no provider-specific switch statement scattered across
// the service layer.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

// SettingsGetter is the minimal slice of the settings store that providers need
// (e.g. GitHub App credentials). Declared here so the package never imports the
// store package directly; *store.SettingStore satisfies it.
type SettingsGetter interface {
	Get(ctx context.Context, key string) (string, error)
}

// GitRepo represents a repository returned by a git provider.
type GitRepo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// GitProvider abstracts a git hosting integration (GitHub, GitLab, Gitea, …).
// Each implementation parses the credential shape it expects from the resource's
// raw JSON config.
type GitProvider interface {
	// Name is the stable provider key stored on the resource (e.g. "github").
	Name() string
	// TestConnection validates the stored credentials. The string is a
	// human-readable status suitable for surfacing in the UI. err is reserved
	// for unexpected failures; credential problems are reported via (false, msg).
	TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error)
	// ListRepos lists repositories the credentials can access.
	ListRepos(ctx context.Context, cfg json.RawMessage) ([]GitRepo, error)
	// Refresh returns an updated config when credentials were rotated (e.g. an
	// OAuth access token refreshed before expiry). changed reports whether the
	// caller should persist the returned config. A nil error with changed=false
	// means nothing to do; errors are non-fatal hints for the caller to log.
	Refresh(ctx context.Context, cfg json.RawMessage) (updated json.RawMessage, changed bool, err error)
	// CloneToken returns a fresh token usable to clone repoURL. Implementations
	// may mint a short-lived token (GitHub App installation token) or fall back
	// to the stored credential.
	CloneToken(ctx context.Context, repoURL string, cfg json.RawMessage) (string, error)
	// VerifyWebhook reports whether an inbound webhook request is authentic.
	VerifyWebhook(secret string, body []byte, headers http.Header) bool
}

// ObjectInfo describes one object returned by ObjectStorageProvider.List.
type ObjectInfo struct {
	Key          string
	FileName     string
	SizeBytes    int64
	LastModified string
}

// ObjectStorageProvider abstracts S3-compatible object storage (AWS S3, MinIO,
// R2, …). Host-side operations use the AWS SDK; in-cluster backup/restore Jobs
// consume UploadJob/DownloadJob specs.
type ObjectStorageProvider interface {
	Name() string
	Bucket(cfg json.RawMessage) (string, error)
	TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error)
	Upload(ctx context.Context, cfg json.RawMessage, localPath, key string) error
	Download(ctx context.Context, cfg json.RawMessage, key, localPath string) error
	List(ctx context.Context, cfg json.RawMessage, prefix string) ([]ObjectInfo, error)
	Delete(ctx context.Context, cfg json.RawMessage, key string) error
	UploadJob(cfg json.RawMessage, srcPath, key string) (orchestrator.ObjectTransfer, error)
	DownloadJob(cfg json.RawMessage, key, destPath string) (orchestrator.ObjectTransfer, error)
}

// RegistryProvider abstracts a container registry integration (ECR, Docker Hub,
// GHCR, custom).
type RegistryProvider interface {
	// Name is the stable provider key stored on the resource (e.g. "ecr").
	Name() string
	// DockerAuth resolves the registry host and credentials used to build a
	// Kubernetes .dockerconfigjson imagePullSecret entry.
	DockerAuth(ctx context.Context, cfg json.RawMessage) (host, username, password string, err error)
	// ShortLived reports whether DockerAuth returns expiring tokens that must be
	// periodically refreshed (true for ECR).
	ShortLived() bool
	// TestConnection validates the stored credentials.
	TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error)
}

// ErrUnsupportedProvider is returned by the registry when no implementation is
// registered for a given provider key. Its message intentionally matches the
// legacy "unsupported provider: X" wording.
type ErrUnsupportedProvider struct {
	Provider string
}

func (e ErrUnsupportedProvider) Error() string {
	return fmt.Sprintf("unsupported provider: %s", e.Provider)
}
