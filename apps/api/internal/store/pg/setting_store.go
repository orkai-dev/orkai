package pg

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
)

type settingStore struct {
	db      bun.IDB
	secrets secret.Store
}

func (s *settingStore) Get(ctx context.Context, key string) (string, error) {
	setting := new(model.Setting)
	err := s.db.NewSelect().Model(setting).Where("key = ?", key).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	if model.IsSensitiveSettingKey(key) {
		return secret.DecryptOptional(s.secrets, setting.Value)
	}
	return setting.Value, nil
}

func (s *settingStore) Set(ctx context.Context, key, value string) error {
	stored := value
	if model.IsSensitiveSettingKey(key) && value != "" {
		var err error
		stored, err = s.secrets.Encrypt(value)
		if err != nil {
			return err
		}
	}
	_, err := s.db.NewInsert().
		Model(&model.Setting{Key: key, Value: stored}).
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_at = NOW()").
		Exec(ctx)
	return err
}

func (s *settingStore) GetAll(ctx context.Context) ([]model.Setting, error) {
	var settings []model.Setting
	err := s.db.NewSelect().Model(&settings).OrderExpr("key ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	for i := range settings {
		if !model.IsSensitiveSettingKey(settings[i].Key) {
			continue
		}
		plain, err := secret.DecryptOptional(s.secrets, settings[i].Value)
		if err != nil {
			return nil, err
		}
		settings[i].Value = plain
	}
	return settings, nil
}
