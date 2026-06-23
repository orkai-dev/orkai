package pg

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

type apiKeyStore struct {
	db bun.IDB
}

func (s *apiKeyStore) Create(ctx context.Context, key *model.APIKey) error {
	_, err := s.db.NewInsert().Model(key).Returning("*").Exec(ctx)
	return err
}

func (s *apiKeyStore) GetByHash(ctx context.Context, hash string) (*model.APIKey, error) {
	key := new(model.APIKey)
	err := s.db.NewSelect().
		Model(key).
		Where("key_hash = ?", hash).
		Scan(ctx)
	return key, err
}

func (s *apiKeyStore) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*model.APIKey, error) {
	key := new(model.APIKey)
	err := s.db.NewSelect().
		Model(key).
		Where("id = ?", id).
		Where("user_id = ?", userID).
		Scan(ctx)
	return key, err
}

func (s *apiKeyStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.APIKey, error) {
	var keys []model.APIKey
	err := s.db.NewSelect().
		Model(&keys).
		Where("user_id = ?", userID).
		OrderExpr("created_at DESC").
		Scan(ctx)
	return keys, err
}

func (s *apiKeyStore) Delete(ctx context.Context, id, userID uuid.UUID) error {
	res, err := s.db.NewDelete().
		Model((*model.APIKey)(nil)).
		Where("id = ?", id).
		Where("user_id = ?", userID).
		Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *apiKeyStore) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := s.db.NewUpdate().
		Model((*model.APIKey)(nil)).
		Set("last_used_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *apiKeyStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*model.APIKey)(nil)).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}
