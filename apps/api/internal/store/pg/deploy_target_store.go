package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

type deployTargetStore struct {
	db bun.IDB
}

func (s *deployTargetStore) GetByID(ctx context.Context, id uuid.UUID) (*model.DeployTarget, error) {
	rec := new(model.DeployTarget)
	err := s.db.NewSelect().
		Model(rec).
		Where("id = ?", id).
		Scan(ctx)
	return rec, err
}

func (s *deployTargetStore) GetDefault(ctx context.Context) (*model.DeployTarget, error) {
	rec := new(model.DeployTarget)
	err := s.db.NewSelect().
		Model(rec).
		Where("is_default = ?", true).
		Limit(1).
		Scan(ctx)
	return rec, err
}

func (s *deployTargetStore) List(ctx context.Context) ([]model.DeployTarget, error) {
	var recs []model.DeployTarget
	err := s.db.NewSelect().
		Model(&recs).
		OrderExpr("created_at ASC").
		Scan(ctx)
	return recs, err
}

func (s *deployTargetStore) Create(ctx context.Context, rec *model.DeployTarget) error {
	_, err := s.db.NewInsert().Model(rec).Returning("*").Exec(ctx)
	return err
}

func (s *deployTargetStore) Update(ctx context.Context, rec *model.DeployTarget) error {
	_, err := s.db.NewUpdate().Model(rec).WherePK().Returning("*").Exec(ctx)
	return err
}
