// Package pages centralizes static-site hosting providers (AWS CloudFront + S3
// today; Cloudflare Pages later) behind a single PagesProvider interface plus a
// registry. Anything that talks to a cloud hosting backend goes through this
// interface — services never call the AWS SDK directly.
package pages

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

// Auth modes for a cloud account's Credentials.
const (
	// AuthAccessKey uses the static AccessKeyID/SecretAccessKey below. Default
	// when AuthMode is empty (backward compatibility).
	AuthAccessKey = "access_key"
	// AuthInstanceRole uses the AWS default credential chain: environment
	// variables + the EC2 instance's attached IAM role.
	AuthInstanceRole = "instance_role"
	// AuthAssumeRole assumes RoleARN via STS. The base credentials used to make
	// the AssumeRole call come from the static keys when provided, otherwise the
	// default chain (e.g. the EC2 instance profile). This lets a deployment
	// assume an IAM role distinct from its instance profile.
	AuthAssumeRole = "assume_role"
)

// Credentials are the cloud-account credentials a provider uses to provision and
// manage infrastructure on the customer's behalf. Parsed from a
// `cloud_account` shared resource's JSON config.
type Credentials struct {
	// AuthMode selects how credentials are resolved: "access_key" (default —
	// static keys below), "instance_role" (the AWS default credential chain:
	// environment variables + the EC2 instance's attached IAM role), or
	// "assume_role" (assume RoleARN via STS).
	AuthMode        string `json:"auth_mode"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	// RoleARN is the IAM role to assume when AuthMode is "assume_role".
	RoleARN string `json:"role_arn"`
	// ExternalID is the optional STS ExternalId passed when assuming RoleARN
	// (commonly required for cross-account roles).
	ExternalID    string `json:"external_id"`
	DefaultRegion string `json:"default_region"`
}

// UseStaticKeys reports whether static access keys are the primary credential
// source. An empty AuthMode defaults to access-key mode for backward
// compatibility.
func (c Credentials) UseStaticKeys() bool {
	return c.AuthMode == "" || c.AuthMode == AuthAccessKey
}

// UseAssumeRole reports whether RoleARN should be assumed via STS.
func (c Credentials) UseAssumeRole() bool { return c.AuthMode == AuthAssumeRole }

// HasStaticBaseKeys reports whether static keys are present to use as the base
// credentials for an AssumeRole call (otherwise the default chain is used).
func (c Credentials) HasStaticBaseKeys() bool {
	return c.AccessKeyID != "" && c.SecretAccessKey != ""
}

// Validate checks the credentials are internally consistent for the chosen mode.
func (c Credentials) Validate() error {
	if c.UseStaticKeys() && (c.AccessKeyID == "" || c.SecretAccessKey == "") {
		return fmt.Errorf("access key ID and secret access key are required for access-key mode")
	}
	if c.UseAssumeRole() && c.RoleARN == "" {
		return fmt.Errorf("role ARN is required for assume-role mode")
	}
	return nil
}

// ParseCredentials unmarshals and validates AWS pages credentials from a cloud
// account config blob.
func ParseCredentials(cfg json.RawMessage) (Credentials, error) {
	var creds Credentials
	if len(cfg) == 0 {
		return creds, fmt.Errorf("empty cloud account config")
	}
	if err := json.Unmarshal(cfg, &creds); err != nil {
		return creds, fmt.Errorf("parse cloud account config: %w", err)
	}
	if err := creds.Validate(); err != nil {
		return creds, err
	}
	return creds, nil
}

// DeployResult summarizes a single Deploy (sync + invalidation).
type DeployResult struct {
	ProviderRef   string // e.g. CloudFront invalidation ID
	DefaultURL    string // e.g. https://d123.cloudfront.net
	UploadedCount int
	DeletedCount  int
}

// SaveRuntime persists a Page's runtime state incrementally. Providers call it
// after creating each cloud resource so a mid-provision crash can resume without
// duplicating resources.
type SaveRuntime func(ctx context.Context, rt *model.PageRuntime) error

// PagesProvider abstracts a static-site hosting backend.
type PagesProvider interface {
	// Name is the stable provider key stored on the Page (e.g. "aws_cloudfront").
	Name() string
	// TestConnection validates credentials. err is reserved for unexpected
	// failures; credential problems are reported via (false, msg, nil).
	TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error)
	// Provision creates (or reuses, if already recorded in page.Runtime) the
	// cloud resources for the Page. It must be idempotent and persist runtime
	// state incrementally via save. Returns the final runtime state.
	Provision(ctx context.Context, page *model.Page, cfg json.RawMessage, tags map[string]string, save SaveRuntime) (*model.PageRuntime, error)
	// Deploy syncs the contents of filesDir to the Page's origin (with delete of
	// removed files) and invalidates the CDN. onLog streams progress lines.
	Deploy(ctx context.Context, page *model.Page, cfg json.RawMessage, filesDir string, onLog func(string)) (*DeployResult, error)
	// Delete tears down the cloud resources for the Page (best-effort).
	Delete(ctx context.Context, page *model.Page, cfg json.RawMessage) error
}

// CustomDomainAttacher is implemented by providers that register a custom domain
// on the hosting backend (e.g. Cloudflare Pages).
type CustomDomainAttacher interface {
	AttachCustomDomain(ctx context.Context, cfg json.RawMessage, projectName, domain string) error
}

// Registry resolves a PagesProvider by its key.
type Registry struct {
	providers map[string]PagesProvider
}

// NewRegistry builds a registry from the given providers, keyed by Name().
func NewRegistry(provs ...PagesProvider) *Registry {
	m := make(map[string]PagesProvider, len(provs))
	for _, p := range provs {
		m[p.Name()] = p
	}
	return &Registry{providers: m}
}

// Get returns the provider for key, or an error if none is registered.
func (r *Registry) Get(key string) (PagesProvider, error) {
	if key == "" {
		key = string(model.PageProviderAWSCloudFront)
	}
	p, ok := r.providers[key]
	if !ok {
		return nil, fmt.Errorf("unsupported pages provider: %s", key)
	}
	return p, nil
}
