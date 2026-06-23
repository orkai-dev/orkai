package pg

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
)

type notificationChannelStore struct {
	db      bun.IDB
	secrets secret.Store
}

func (s *notificationChannelStore) encryptChannel(ch *model.NotificationChannel) error {
	if ch == nil || len(ch.Config) == 0 {
		return nil
	}
	enc, err := encryptNotificationConfig(s.secrets, ch.Config)
	if err != nil {
		return err
	}
	ch.Config = enc
	return nil
}

func (s *notificationChannelStore) decryptChannel(ch *model.NotificationChannel) error {
	if ch == nil || len(ch.Config) == 0 {
		return nil
	}
	dec, err := decryptNotificationConfig(s.secrets, ch.Config)
	if err != nil {
		return err
	}
	ch.Config = dec
	return nil
}

func (s *notificationChannelStore) GetByOrgAndType(ctx context.Context, orgID uuid.UUID, channelType string) (*model.NotificationChannel, error) {
	ch := new(model.NotificationChannel)
	err := s.db.NewSelect().Model(ch).
		Where("org_id = ?", orgID).
		Where("type = ?", channelType).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.decryptChannel(ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *notificationChannelStore) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	err := s.db.NewSelect().Model(&channels).
		Where("org_id = ?", orgID).
		OrderExpr("type ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	for i := range channels {
		if err := s.decryptChannel(&channels[i]); err != nil {
			return nil, err
		}
	}
	return channels, nil
}

func (s *notificationChannelStore) ListAllEnabled(ctx context.Context) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	err := s.db.NewSelect().Model(&channels).
		Where("enabled = true").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	for i := range channels {
		if err := s.decryptChannel(&channels[i]); err != nil {
			return nil, err
		}
	}
	return channels, nil
}

func (s *notificationChannelStore) Upsert(ctx context.Context, channel *model.NotificationChannel) error {
	if err := s.encryptChannel(channel); err != nil {
		return err
	}

	existing := new(model.NotificationChannel)
	err := s.db.NewSelect().Model(existing).
		Where("org_id = ?", channel.OrgID).
		Where("type = ?", channel.Type).
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		_, err = s.db.NewInsert().Model(channel).Returning("*").Exec(ctx)
		if err != nil {
			return err
		}
		return s.decryptChannel(channel)
	}

	existing.Enabled = channel.Enabled
	existing.Config = channel.Config
	_, err = s.db.NewUpdate().Model(existing).
		Set("enabled = ?", channel.Enabled).
		Set("config = ?", channel.Config).
		Set("updated_at = NOW()").
		WherePK().
		Returning("*").
		Exec(ctx)
	if err != nil {
		return err
	}
	*channel = *existing
	return s.decryptChannel(channel)
}
