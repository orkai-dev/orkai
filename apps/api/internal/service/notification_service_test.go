package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNotificationService(fs *testsupport.FakeStore) *NotificationService {
	settings := newSettingService(fs, testsupport.NewFakeTargetRegistry())
	return NewNotificationService(fs, settings, testsupport.NewTestLogger())
}

func TestNotifGetChannel(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.GetByOrgAndTypeFn = func(ctx context.Context, orgID uuid.UUID, ct string) (*model.NotificationChannel, error) {
		return &model.NotificationChannel{Type: model.NotifySlack}, nil
	}
	s := newNotificationService(fs)
	ch, err := s.GetChannel(context.Background(), uuid.New(), "slack")
	require.NoError(t, err)
	assert.Equal(t, model.NotifySlack, ch.Type)
}

func TestNotifListChannels(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
		return []model.NotificationChannel{{}}, nil
	}
	s := newNotificationService(fs)
	chs, err := s.ListChannels(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, chs, 1)
}

func TestNotifSaveChannel(t *testing.T) {
	fs := testsupport.NewFakeStore()
	var saved *model.NotificationChannel
	fs.NotificationChannelStore.UpsertFn = func(ctx context.Context, ch *model.NotificationChannel) error {
		saved = ch
		return nil
	}
	s := newNotificationService(fs)
	require.NoError(t, s.SaveChannel(context.Background(), uuid.New(), "slack", true, json.RawMessage(`{}`)))
	require.NotNil(t, saved)
	assert.True(t, saved.Enabled)
}

func TestNotifTestChannelNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.GetByOrgAndTypeFn = func(ctx context.Context, orgID uuid.UUID, ct string) (*model.NotificationChannel, error) {
		return nil, errors.New("missing")
	}
	s := newNotificationService(fs)
	require.Error(t, s.TestChannel(context.Background(), uuid.New(), "slack"))
}

func TestNotifyListError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
		return nil, errors.New("query failed")
	}
	s := newNotificationService(fs)
	require.Error(t, s.Notify(context.Background(), uuid.New(), model.EventDeploySuccess, "t", "m"))
}

func TestNotifySkipsDisabledAndSends(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
		return []model.NotificationChannel{
			{Type: model.NotifySlack, Enabled: false},
			{Type: model.NotifySlack, Enabled: true, Config: json.RawMessage(`{"webhook_url":"http://localhost/x"}`)},
		}, nil
	}
	s := newNotificationService(fs)
	// blocked URL means sendToChannel errors are logged, but Notify returns nil
	require.NoError(t, s.Notify(context.Background(), uuid.New(), model.EventDeployFailed, "t", "m"))
}

func TestNotifyAllOrgs(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListAllEnabledFn = func(ctx context.Context) ([]model.NotificationChannel, error) {
		return []model.NotificationChannel{
			{Type: model.NotifyDiscord, Config: json.RawMessage(`{"webhook_url":"http://localhost/x"}`)},
		}, nil
	}
	s := newNotificationService(fs)
	s.NotifyAllOrgs(context.Background(), model.EventDeploySuccess, "t", "m")
}

func TestNotifyAllOrgsError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListAllEnabledFn = func(ctx context.Context) ([]model.NotificationChannel, error) {
		return nil, errors.New("query failed")
	}
	s := newNotificationService(fs)
	s.NotifyAllOrgs(context.Background(), model.EventDeploySuccess, "t", "m")
}

func TestNotifyAsyncAndShutdown(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
		return nil, nil
	}
	fs.NotificationChannelStore.ListAllEnabledFn = func(ctx context.Context) ([]model.NotificationChannel, error) {
		return nil, nil
	}
	s := newNotificationService(fs)
	s.NotifyAsync(uuid.New(), model.EventDeploySuccess, "t", "m")
	s.NotifyAllOrgsAsync(model.EventDeploySuccess, "t", "m")
	s.Shutdown()
}

func TestEventSeverity(t *testing.T) {
	assert.Equal(t, "critical", eventSeverity(model.EventDeployFailed))
	assert.Equal(t, "info", eventSeverity(model.EventDeploySuccess))
	assert.Equal(t, "warning", eventSeverity(model.EventDeployCancelled))
}

func TestChannelAllowsEventAbsentConfig(t *testing.T) {
	ch := &model.NotificationChannel{Config: json.RawMessage(`{"webhook_url":"http://x"}`)}
	assert.True(t, channelAllowsEvent(ch, model.EventDeploySuccess))
}

func TestChannelAllowsEventListed(t *testing.T) {
	ch := &model.NotificationChannel{
		Config: json.RawMessage(`{"events":["deploy_success","app_deleted"]}`),
	}
	assert.True(t, channelAllowsEvent(ch, model.EventDeploySuccess))
	assert.True(t, channelAllowsEvent(ch, model.EventAppDeleted))
	assert.False(t, channelAllowsEvent(ch, model.EventDeployFailed))
}

func TestChannelAllowsEventEmptyList(t *testing.T) {
	ch := &model.NotificationChannel{Config: json.RawMessage(`{"events":[]}`)}
	assert.False(t, channelAllowsEvent(ch, model.EventDeploySuccess))
}

func TestNotifySkipsFilteredEvents(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.NotificationChannelStore.ListByOrgFn = func(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
		return []model.NotificationChannel{
			{
				Type:    model.NotifySlack,
				Enabled: true,
				Config:  json.RawMessage(`{"webhook_url":"http://localhost/x","events":["deploy_failed"]}`),
			},
		}, nil
	}
	s := newNotificationService(fs)
	require.NoError(t, s.Notify(context.Background(), uuid.New(), model.EventDeploySuccess, "t", "m"))
	require.NoError(t, s.Notify(context.Background(), uuid.New(), model.EventDeployFailed, "t", "m"))
}

func TestIsBlockedURL(t *testing.T) {
	assert.True(t, isBlockedURL("http://localhost/x"))
	assert.True(t, isBlockedURL("http://127.0.0.1:8080/x"))
	assert.True(t, isBlockedURL("http://10.0.0.1/x"))
	assert.True(t, isBlockedURL("http://192.168.1.1/x"))
	assert.True(t, isBlockedURL("https://kubernetes.default/x"))
	assert.False(t, isBlockedURL("https://hooks.slack.com/services/x"))
}

func TestSendToChannelUnsupported(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendToChannel(context.Background(), &model.NotificationChannel{Type: model.NotificationChannelType("carrier-pigeon")}, "t", "m", "info")
	require.ErrorContains(t, err, "unsupported channel type")
}

func TestSendTelegramMissingConfig(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendTelegram(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{}`)}, "t", "m")
	require.ErrorContains(t, err, "required")
}

func TestSendDiscordMissingConfig(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendDiscord(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{}`)}, "t", "m", "info")
	require.ErrorContains(t, err, "required")
}

func TestSendDiscordBlocked(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendDiscord(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{"webhook_url":"http://localhost/x"}`)}, "t", "m", "info")
	require.ErrorContains(t, err, "blocked")
}

func TestSendSlackMissingConfig(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendSlack(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{}`)}, "t", "m", "info")
	require.ErrorContains(t, err, "required")
}

func TestSendSlackBlocked(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendSlack(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{"webhook_url":"http://localhost/x"}`)}, "t", "m", "info")
	require.ErrorContains(t, err, "blocked")
}

func TestSendGoogleChatMissingConfig(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendGoogleChat(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{}`)}, "t", "m", "info")
	require.ErrorContains(t, err, "required")
}

func TestSendGoogleChatBlocked(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendGoogleChat(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{"webhook_url":"http://localhost/x"}`)}, "t", "m", "info")
	require.ErrorContains(t, err, "blocked")
}

func TestSendEmailNotConfigured(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	err := s.sendEmail(context.Background(), &model.NotificationChannel{Config: json.RawMessage(`{}`)}, "t", "m", "info")
	require.ErrorContains(t, err, "not configured")
}

func TestNotifSMTPConfigPassthrough(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newNotificationService(fs)
	cfg, err := s.GetSMTPConfig(context.Background())
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	require.NoError(t, s.SaveSMTPConfig(context.Background(), &SMTPConfig{Host: "smtp.example.com", Port: "587", From: "a@b.c"}))
}

func TestTestSMTPNotConfigured(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	require.Error(t, s.TestSMTP(context.Background()))
}

func TestSendInvitationEmailNotConfigured(t *testing.T) {
	s := newNotificationService(testsupport.NewFakeStore())
	require.Error(t, s.SendInvitationEmail(context.Background(), "a@b.c", "member", "http://x"))
}
