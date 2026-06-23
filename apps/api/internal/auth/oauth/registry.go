package oauth

import (
	"context"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// ProviderStatus describes whether a login provider is available.
type ProviderStatus struct {
	Enabled bool `json:"enabled"`
}

// Registry resolves enabled OAuth login providers from settings.
type Registry struct {
	settings store.SettingStore
}

func NewRegistry(settings store.SettingStore) *Registry {
	return &Registry{settings: settings}
}

func (r *Registry) ListProviders(ctx context.Context) map[string]ProviderStatus {
	return map[string]ProviderStatus{
		GoogleProviderName: {Enabled: r.isGoogleEnabled(ctx)},
	}
}

func (r *Registry) GetProvider(ctx context.Context, name string) (Provider, error) {
	switch name {
	case GoogleProviderName:
		if !r.isGoogleEnabled(ctx) {
			return nil, ErrProviderDisabled
		}
		clientID, _ := r.settings.Get(ctx, model.SettingGoogleOAuthClientID)
		clientSecret, _ := r.settings.Get(ctx, model.SettingGoogleOAuthClientSecret)
		if clientID == "" || clientSecret == "" {
			return nil, ErrProviderNotConfigured
		}
		return NewGoogleProvider(clientID, clientSecret), nil
	default:
		return nil, ErrProviderUnknown
	}
}

func (r *Registry) isGoogleEnabled(ctx context.Context) bool {
	enabled, _ := r.settings.Get(ctx, model.SettingGoogleOAuthEnabled)
	if enabled != "true" {
		return false
	}
	clientID, _ := r.settings.Get(ctx, model.SettingGoogleOAuthClientID)
	clientSecret, _ := r.settings.Get(ctx, model.SettingGoogleOAuthClientSecret)
	return clientID != "" && clientSecret != ""
}
