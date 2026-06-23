package model

import (
	"strings"
	"time"
)

// SettingSecretMask is the placeholder returned in API responses in place of a
// stored secret value. Saving a setting back with this exact value is treated
// as "unchanged" so the real secret is preserved.
const SettingSecretMask = "••••••••"

// IsSensitiveSettingKey reports whether a setting key holds a secret that must
// never be returned to clients in plaintext.
func IsSensitiveSettingKey(key string) bool {
	switch {
	case strings.HasSuffix(key, "_secret"),
		strings.HasSuffix(key, "_password"),
		strings.HasSuffix(key, "_pem"),
		strings.HasSuffix(key, "_token"),
		strings.HasPrefix(key, "github_setup_state_"),
		strings.HasPrefix(key, "github_oauth_state_"):
		return true
	default:
		return false
	}
}

// Setting stores global key-value configuration in the database.
type Setting struct {
	Key       string    `bun:"key,pk" json:"key"`
	Value     string    `bun:"value,notnull" json:"value"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
}

// Well-known setting keys
const (
	SettingBaseDomain  = "base_domain"  // e.g. "203.0.113.5.sslip.io" or "mysite.com"
	SettingServerIP    = "server_ip"    // auto-detected server IP
	SettingSetupDone   = "setup_done"   // "true" after first-time setup
	SettingPanelDomain = "panel_domain" // domain for the Orkai panel (e.g. "panel.example.com")
	SettingHTTPSEmail  = "https_email"  // email for Let's Encrypt ACME certificates

	// OAuth login (SSO)
	SettingGoogleOAuthEnabled      = "google_oauth_enabled"
	SettingGoogleOAuthClientID     = "google_oauth_client_id"
	SettingGoogleOAuthClientSecret = "google_oauth_client_secret"
	SettingOAuthAllowedDomains     = "oauth_allowed_domains" // comma-separated; empty = no restriction
	SettingAuthGoogleOnly          = "auth_google_only"      // "true" = passwordless Google-only mode

	// VersionInfo is JSON-serialized VersionInfo written by the leader replica.
	SettingVersionInfo = "version_info"
)
