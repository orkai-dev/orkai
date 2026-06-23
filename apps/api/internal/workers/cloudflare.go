// Package workers contains the Cloudflare Workers integration helpers shared by
// the worker service and deploy service. There is a single provider
// (Cloudflare) so no registry abstraction is needed; credential parsing and the
// connection test reuse the existing internal/cloudflare client.
package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/orkai-dev/orkai/apps/api/internal/cloudflare"
)

// ParseCredentials decodes a cloud_account shared resource's JSON config into
// Cloudflare credentials and validates them.
func ParseCredentials(cfg json.RawMessage) (cloudflare.Credentials, error) {
	var creds cloudflare.Credentials
	if len(cfg) == 0 {
		return creds, fmt.Errorf("cloud account has no credentials configured")
	}
	if err := json.Unmarshal(cfg, &creds); err != nil {
		return creds, fmt.Errorf("invalid cloudflare credentials: %w", err)
	}
	if err := creds.Validate(); err != nil {
		return creds, err
	}
	return creds, nil
}

// TestConnection validates a cloud account's Cloudflare credentials.
func TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	creds, err := ParseCredentials(cfg)
	if err != nil {
		return false, err.Error(), nil
	}
	return cloudflare.TestConnection(ctx, creds)
}

// DeployCredentials resolves the API token + account id wrangler needs to
// deploy. Workers require a scoped API token (Workers Scripts:Edit); the global
// API key + email auth mode is not supported for deploys.
func DeployCredentials(cfg json.RawMessage) (apiToken, accountID string, err error) {
	creds, perr := ParseCredentials(cfg)
	if perr != nil {
		return "", "", perr
	}
	if !creds.UseAPIToken() || creds.APIToken == "" {
		return "", "", fmt.Errorf("worker deploys require a Cloudflare API token (Workers Scripts:Edit); the global API key auth mode is not supported")
	}
	return creds.APIToken, creds.AccountID, nil
}
