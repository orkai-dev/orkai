//go:build integration

package pg_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
)

func TestIntegrationSecretsAtRest(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	plainToken := "ghp_super_secret_token"
	require.NoError(t, st.Settings().Set(ctx, model.SettingGoogleOAuthClientSecret, plainToken))

	var raw string
	err := st.DB().NewSelect().
		TableExpr("settings").
		Column("value").
		Where("key = ?", model.SettingGoogleOAuthClientSecret).
		Scan(ctx, &raw)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(raw, "enc:v1:"))
	assert.NotContains(t, raw, plainToken)

	got, err := st.Settings().Get(ctx, model.SettingGoogleOAuthClientSecret)
	require.NoError(t, err)
	assert.Equal(t, plainToken, got)

	org := &model.Organization{Name: "SecretCo"}
	require.NoError(t, st.Organizations().Create(ctx, org))

	cfg, _ := json.Marshal(map[string]string{
		"token": plainToken,
		"name":  "my-repo",
	})
	resource := &model.SharedResource{
		OrgID:    org.ID,
		Name:     "gh",
		Type:     model.ResourceGitProvider,
		Provider: "github",
		Config:   cfg,
	}
	require.NoError(t, st.SharedResources().Create(ctx, resource))

	var rawConfig string
	err = st.DB().NewSelect().
		TableExpr("shared_resources").
		Column("config").
		Where("id = ?", resource.ID).
		Scan(ctx, &rawConfig)
	require.NoError(t, err)
	assert.Contains(t, rawConfig, "enc:v1:")
	assert.NotContains(t, rawConfig, plainToken)

	gotResource, err := st.SharedResources().GetByID(ctx, resource.ID)
	require.NoError(t, err)
	var parsed map[string]string
	require.NoError(t, json.Unmarshal(gotResource.Config, &parsed))
	assert.Equal(t, plainToken, parsed["token"])
	assert.Equal(t, "my-repo", parsed["name"])

	user := &model.User{
		OrgID:        org.ID,
		Email:        "2fa@example.com",
		PasswordHash: "hash",
		TwoFASecret:  "JBSWY3DPEHPK3PXP",
	}
	require.NoError(t, st.Users().Create(ctx, user))

	var raw2FA string
	err = st.DB().NewSelect().
		TableExpr("users").
		Column("two_fa_secret").
		Where("id = ?", user.ID).
		Scan(ctx, &raw2FA)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(raw2FA, "enc:v1:"))

	gotUser, err := st.Users().GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", gotUser.TwoFASecret)

	// Sanity: wrong setup secret cannot decrypt.
	wrong := secret.NewFromSetupSecret("wrong-setup-secret-32-chars-long!!")
	_, err = wrong.Decrypt(raw)
	assert.Error(t, err)
}

func TestIntegrationApplicationSecretsAtRest(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	org := &model.Organization{Name: "AppCo"}
	require.NoError(t, st.Organizations().Create(ctx, org))
	team := &model.Team{OrgID: org.ID, Name: "t"}
	require.NoError(t, st.Teams().Create(ctx, team))
	project := &model.Project{OrgID: org.ID, TeamID: team.ID, Name: "p", Environment: model.EnvProd}
	require.NoError(t, st.Projects().Create(ctx, project))

	app := &model.Application{
		ProjectID:     project.ID,
		Name:          "api",
		WebhookSecret: "hook-secret",
		Secrets:       map[string]string{"API_KEY": "key123"},
	}
	require.NoError(t, st.Applications().Create(ctx, app))

	var rawHook string
	err := st.DB().NewSelect().
		TableExpr("applications").
		Column("webhook_secret").
		Where("id = ?", app.ID).
		Scan(ctx, &rawHook)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(rawHook, "enc:v1:"))

	var rawSecrets string
	err = st.DB().NewSelect().
		TableExpr("applications").
		Column("secrets").
		Where("id = ?", app.ID).
		Scan(ctx, &rawSecrets)
	require.NoError(t, err)
	assert.Contains(t, rawSecrets, "enc:v1:")
	assert.NotContains(t, rawSecrets, "key123")

	got, err := st.Applications().GetByID(ctx, app.ID)
	require.NoError(t, err)
	assert.Equal(t, "hook-secret", got.WebhookSecret)
	assert.Equal(t, "key123", got.Secrets["API_KEY"])
}

func TestIntegrationNotificationSecretsAtRest(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	org := &model.Organization{Name: "NotifyCo"}
	require.NoError(t, st.Organizations().Create(ctx, org))

	cfg, _ := json.Marshal(map[string]string{
		"bot_token": "123456:ABC-DEF",
		"chat_id":   "999",
	})
	require.NoError(t, st.NotificationChannels().Upsert(ctx, &model.NotificationChannel{
		OrgID:   org.ID,
		Type:    model.NotifyTelegram,
		Enabled: true,
		Config:  cfg,
	}))

	var rawConfig string
	err := st.DB().NewSelect().
		TableExpr("notification_channels").
		Column("config").
		Where("org_id = ?", org.ID).
		Scan(ctx, &rawConfig)
	require.NoError(t, err)
	assert.Contains(t, rawConfig, "enc:v1:")
	assert.NotContains(t, rawConfig, "123456:ABC-DEF")

	ch, err := st.NotificationChannels().GetByOrgAndType(ctx, org.ID, string(model.NotifyTelegram))
	require.NoError(t, err)
	var parsed map[string]string
	require.NoError(t, json.Unmarshal(ch.Config, &parsed))
	assert.Equal(t, "123456:ABC-DEF", parsed["bot_token"])
}

func TestIntegrationServerNodePasswordAtRest(t *testing.T) {
	st, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	node := &model.ServerNode{
		Name:     "worker-1",
		Host:     "10.0.0.5",
		Password: "ssh-pass",
	}
	require.NoError(t, st.ServerNodes().Create(ctx, node))

	var raw string
	err := st.DB().NewSelect().
		TableExpr("server_nodes").
		Column("password").
		Where("id = ?", node.ID).
		Scan(ctx, &raw)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(raw, "enc:v1:"))

	got, err := st.ServerNodes().GetByID(ctx, node.ID)
	require.NoError(t, err)
	assert.Equal(t, "ssh-pass", got.Password)
}
