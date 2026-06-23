package pg

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type applicationStore struct {
	db      bun.IDB
	secrets secret.Store
}

func (s *applicationStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	app := new(model.Application)
	err := s.db.NewSelect().
		Model(app).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptApplicationSecrets(s.secrets, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *applicationStore) Create(ctx context.Context, app *model.Application) error {
	if err := encryptApplicationSecrets(s.secrets, app); err != nil {
		return err
	}
	_, err := s.db.NewInsert().Model(app).Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptApplicationSecrets(s.secrets, app)
}

func (s *applicationStore) Update(ctx context.Context, app *model.Application) error {
	if err := encryptApplicationSecrets(s.secrets, app); err != nil {
		return err
	}
	_, err := s.db.NewUpdate().Model(app).WherePK().Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptApplicationSecrets(s.secrets, app)
}

func (s *applicationStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.AppStatus) error {
	_, err := s.db.NewUpdate().
		Model((*model.Application)(nil)).
		Set("status = ?", status).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *applicationStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Application)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *applicationStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Application, int, error) {
	var apps []model.Application
	count, err := s.db.NewSelect().
		Model(&apps).
		Where("project_id = ?", projectID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := decryptApplications(s.secrets, apps); err != nil {
		return nil, 0, err
	}
	return apps, count, nil
}

func (s *applicationStore) ListAll(ctx context.Context, params store.ListParams, filter store.AppListFilter) ([]model.Application, int, error) {
	var apps []model.Application
	if filter.ProjectIDs != nil && len(filter.ProjectIDs) == 0 {
		return apps, 0, nil
	}
	q := s.db.NewSelect().Model(&apps)

	if filter.Search != "" {
		q = q.Where("LOWER(name) LIKE LOWER(?) ESCAPE '\\'", likeContains(filter.Search))
	}
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
	if err != nil {
		return nil, 0, err
	}
	if err := decryptApplications(s.secrets, apps); err != nil {
		return nil, 0, err
	}
	return apps, count, nil
}

func (s *applicationStore) ExistsByK8sName(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error) {
	return s.db.NewSelect().
		Model((*model.Application)(nil)).
		Where("project_id = ?", projectID).
		Where("k8s_name = ?", k8sName).
		Exists(ctx)
}

func (s *applicationStore) FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Application, error) {
	app := new(model.Application)
	err := s.db.NewSelect().
		Model(app).
		Where("git_provider_id = ? OR registry_id = ?", resourceID, resourceID).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return app, nil
}

func (s *applicationStore) ListWithRegistry(ctx context.Context) ([]model.Application, error) {
	var apps []model.Application
	err := s.db.NewSelect().
		Model(&apps).
		Where("registry_id IS NOT NULL").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptApplications(s.secrets, apps); err != nil {
		return nil, err
	}
	return apps, nil
}
