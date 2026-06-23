package cloudflare

import "fmt"

const (
	AuthAPIToken = "api_token"
	AuthAPIKey   = "api_key"
)

// Credentials are parsed from a cloud_account shared resource's JSON config when
// provider is "cloudflare".
type Credentials struct {
	AuthMode  string `json:"auth_mode"`
	APIToken  string `json:"api_token"`
	APIKey    string `json:"api_key"`
	Email     string `json:"email"`
	AccountID string `json:"account_id"`
}

// UseAPIToken reports whether a scoped API token is the auth method.
func (c Credentials) UseAPIToken() bool {
	return c.AuthMode == "" || c.AuthMode == AuthAPIToken
}

// UseAPIKey reports whether global API key + email auth is used.
func (c Credentials) UseAPIKey() bool { return c.AuthMode == AuthAPIKey }

// Validate checks the credentials are internally consistent for the chosen mode.
func (c Credentials) Validate() error {
	if c.UseAPIToken() {
		if c.APIToken == "" {
			return fmt.Errorf("API token is required for api-token mode")
		}
		return nil
	}
	if c.UseAPIKey() {
		if c.APIKey == "" || c.Email == "" {
			return fmt.Errorf("API key and email are required for api-key mode")
		}
		return nil
	}
	return fmt.Errorf("unsupported auth_mode %q (use api_token or api_key)", c.AuthMode)
}
