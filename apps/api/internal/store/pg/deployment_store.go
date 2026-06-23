package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type deploymentStore struct {
	db bun.IDB
}

func (s *deploymentStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	deploy := new(model.Deployment)
	err := s.db.NewSelect().Model(deploy).Where("id = ?", id).Scan(ctx)
	return deploy, err
}

func (s *deploymentStore) Create(ctx context.Context, deploy *model.Deployment) error {
	_, err := s.db.NewInsert().Model(deploy).Returning("*").Exec(ctx)
	return err
}

func (s *deploymentStore) Update(ctx context.Context, deploy *model.Deployment) error {
	_, err := s.db.NewUpdate().Model(deploy).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *deploymentStore) ListByApp(ctx context.Context, appID uuid.UUID, params store.ListParams) ([]model.Deployment, int, error) {
	var deploys []model.Deployment
	count, err := s.db.NewSelect().
		Model(&deploys).
		Where("app_id = ?", appID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deploys, count, err
}

func (s *deploymentStore) ListAll(ctx context.Context, params store.ListParams, filter store.DeploymentListFilter) ([]model.Deployment, int, error) {
	var deploys []model.Deployment
	// A non-nil but empty ProjectIDs scope means the caller has access to no
	// projects — short-circuit to an empty result.
	if filter.ProjectIDs != nil && len(filter.ProjectIDs) == 0 {
		return deploys, 0, nil
	}
	q := s.db.NewSelect().Model(&deploys)

	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if len(filter.ProjectIDs) > 0 {
		q = q.Where("project_id IN (?)", bun.List(filter.ProjectIDs))
	}

	count, err := q.
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deploys, count, err
}

func (s *deploymentStore) GetLatestByApp(ctx context.Context, appID uuid.UUID) (*model.Deployment, error) {
	deploy := new(model.Deployment)
	err := s.db.NewSelect().
		Model(deploy).
		Where("app_id = ?", appID).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	return deploy, err
}
