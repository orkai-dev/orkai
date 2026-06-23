package service

import (
	"context"
	"errors"
	"testing"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSettingService(fs *testsupport.FakeStore, reg *orchestrator.TargetRegistry) *SettingService {
	return NewSettingService(fs, reg, testsupport.NewTestLogger())
}

func TestSettingGetSetGetAll(t *testing.T) {
	fs := testsupport.NewFakeStore()
	store := map[string]string{}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return store[key], nil
	}
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		store[key] = value
		return nil
	}
	fs.SettingsStore.GetAllFn = func(ctx context.Context) ([]model.Setting, error) {
		return []model.Setting{{Key: "a", Value: "b"}}, nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())

	require.NoError(t, s.Set(context.Background(), "foo", "  bar  "))
	assert.Equal(t, "bar", store["foo"], "value should be trimmed")

	v, err := s.Get(context.Background(), "foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", v)

	all, err := s.GetAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestSettingSetPreservesMaskedSecret(t *testing.T) {
	fs := testsupport.NewFakeStore()
	writes := 0
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		writes++
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())

	// Re-saving a sensitive key with the mask sentinel is a no-op (preserved).
	require.NoError(t, s.Set(context.Background(), "smtp_password", model.SettingSecretMask))
	assert.Equal(t, 0, writes)

	// A non-sensitive key with the same value still writes.
	require.NoError(t, s.Set(context.Background(), "base_domain", model.SettingSecretMask))
	assert.Equal(t, 1, writes)
}

func TestSettingSetStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		return errors.New("write failed")
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	err := s.Set(context.Background(), "k", "v")
	require.Error(t, err)
}

func TestSettingSetPanelDomainAppliesIngress(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	called := false
	orch.EnsurePanelIngressFn = func(ctx context.Context, domain, httpsEmail string) error {
		called = true
		assert.Equal(t, "panel.example.com", domain)
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry(orch))
	require.NoError(t, s.Set(context.Background(), model.SettingPanelDomain, "panel.example.com"))
	assert.True(t, called)
}

func TestSettingSetPanelDomainEmptyDeletesIngress(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	deleted := false
	orch.DeletePanelIngressFn = func(ctx context.Context) error {
		deleted = true
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry(orch))
	require.NoError(t, s.Set(context.Background(), model.SettingPanelDomain, ""))
	assert.True(t, deleted)
}

func TestSettingSetPanelDomainIngressError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.EnsurePanelIngressFn = func(ctx context.Context, domain, httpsEmail string) error {
		return errors.New("ingress failed")
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry(orch))
	err := s.Set(context.Background(), model.SettingPanelDomain, "panel.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ingress not applied")
}

func TestSettingSetHTTPSEmailWithPanel(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingPanelDomain {
			return "panel.example.com", nil
		}
		return "", nil
	}
	orch := testsupport.NewFakeOrchestrator()
	called := false
	orch.EnsurePanelIngressFn = func(ctx context.Context, domain, httpsEmail string) error {
		called = true
		assert.Equal(t, "admin@example.com", httpsEmail)
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry(orch))
	require.NoError(t, s.Set(context.Background(), model.SettingHTTPSEmail, "admin@example.com"))
	assert.True(t, called)
}

func TestSettingSetHTTPSEmailNoPanel(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	require.NoError(t, s.Set(context.Background(), model.SettingHTTPSEmail, "admin@example.com"))
}

func TestSettingSetHTTPSEmailError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return "panel.example.com", nil
	}
	orch := testsupport.NewFakeOrchestrator()
	orch.EnsurePanelIngressFn = func(ctx context.Context, domain, httpsEmail string) error {
		return errors.New("boom")
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry(orch))
	err := s.Set(context.Background(), model.SettingHTTPSEmail, "admin@example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS config not applied")
}

func TestSettingGetBaseDomainAndServerIP(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		switch key {
		case model.SettingBaseDomain:
			return "base.example.com", nil
		case "server_ip":
			return "10.0.0.1", nil
		}
		return "", nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	assert.Equal(t, "base.example.com", s.GetBaseDomain(context.Background()))
	assert.Equal(t, "10.0.0.1", s.GetServerIP(context.Background()))
}

func TestSettingInitDefaultsAlreadyDone(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingSetupDone {
			return "true", nil
		}
		return "", nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	require.NoError(t, s.InitDefaults(context.Background()))
}

func TestSettingInitDefaultsDetectsNodeIP(t *testing.T) {
	fs := testsupport.NewFakeStore()
	set := map[string]string{}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return set[key], nil
	}
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		set[key] = value
		return nil
	}
	// The noop default returns a control-plane node with an IP.
	orch := testsupport.NewFakeOrchestrator()
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry(orch))
	require.NoError(t, s.InitDefaults(context.Background()))
	assert.NotEmpty(t, set[model.SettingServerIP])
	assert.Contains(t, set[model.SettingBaseDomain], "sslip.io")
	assert.Equal(t, "true", set[model.SettingSetupDone])
}

func TestSettingReconcileInfra(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	// Should not panic; panel domain empty triggers DeletePanelIngress (noop).
	s.ReconcileInfra(context.Background())
}

func TestSettingSMTPConfigRoundTrip(t *testing.T) {
	fs := testsupport.NewFakeStore()
	set := map[string]string{}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return set[key], nil
	}
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		set[key] = value
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())

	cfg := &SMTPConfig{Host: "smtp.example.com", Port: "587", From: "no-reply@example.com", Enabled: true, Password: "secret"}
	require.NoError(t, s.SaveSMTPConfig(context.Background(), cfg))

	got, err := s.GetSMTPConfig(context.Background())
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, "smtp.example.com", got.Host)
	assert.Equal(t, "secret", got.Password)
}

func TestSettingSMTPConfigValidation(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())

	require.ErrorContains(t, s.SaveSMTPConfig(context.Background(), &SMTPConfig{Enabled: true}), "host is required")
	require.ErrorContains(t, s.SaveSMTPConfig(context.Background(), &SMTPConfig{Enabled: true, Host: "h"}), "port is required")
	require.ErrorContains(t, s.SaveSMTPConfig(context.Background(), &SMTPConfig{Enabled: true, Host: "h", Port: "1"}), "from address is required")
}

func TestSettingSMTPConfigMaskedPasswordKeepsExisting(t *testing.T) {
	fs := testsupport.NewFakeStore()
	set := map[string]string{"smtp_password": "original"}
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return set[key], nil
	}
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		set[key] = value
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())

	cfg := &SMTPConfig{Host: "h", Port: "1", From: "f", Enabled: false, Password: "••••••••"}
	require.NoError(t, s.SaveSMTPConfig(context.Background(), cfg))
	assert.Equal(t, "original", set["smtp_password"])
}

func TestSettingSMTPSaveError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		return errors.New("write failed")
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	err := s.SaveSMTPConfig(context.Background(), &SMTPConfig{Enabled: false})
	require.Error(t, err)
}

func TestSettingSetAuthGoogleOnlyRequiresOAuth(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return "", nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	err := s.Set(context.Background(), model.SettingAuthGoogleOnly, "true")
	require.ErrorContains(t, err, "Google OAuth must be enabled")
}

func TestSettingSetGoogleOAuthDisabledBlockedWhenGoogleOnly(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingAuthGoogleOnly {
			return "true", nil
		}
		return "", nil
	}
	writes := 0
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		writes++
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	err := s.Set(context.Background(), model.SettingGoogleOAuthEnabled, "false")
	require.ErrorContains(t, err, "disable Google-only sign-in")
	assert.Equal(t, 0, writes)
}

func TestSettingSetGoogleOAuthCredentialsBlockedWhenGoogleOnly(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		if key == model.SettingAuthGoogleOnly {
			return "true", nil
		}
		return "", nil
	}
	writes := 0
	fs.SettingsStore.SetFn = func(ctx context.Context, key, value string) error {
		writes++
		return nil
	}
	s := newSettingService(fs, testsupport.NewFakeTargetRegistry())

	err := s.Set(context.Background(), model.SettingGoogleOAuthClientID, "")
	require.ErrorContains(t, err, "disable Google-only sign-in")
	assert.Equal(t, 0, writes)

	err = s.Set(context.Background(), model.SettingGoogleOAuthClientSecret, "")
	require.ErrorContains(t, err, "disable Google-only sign-in")
	assert.Equal(t, 0, writes)
}
