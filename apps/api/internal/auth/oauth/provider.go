package oauth

import (
	"context"
	"errors"
)

const GoogleProviderName = "google"

var (
	ErrProviderUnknown       = errors.New("unknown provider")
	ErrProviderDisabled      = errors.New("oauth disabled")
	ErrProviderNotConfigured = errors.New("oauth not configured")
)

// UserIdentity holds verified identity claims from an OAuth provider.
type UserIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
}

// Provider exchanges an authorization code for user identity claims.
type Provider interface {
	Name() string
	AuthCodeURL(state, redirectURI string) string
	Exchange(ctx context.Context, code, redirectURI string) (*UserIdentity, error)
}
