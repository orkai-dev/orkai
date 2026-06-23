package pg

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type managedDatabaseStore struct {
	db bun.IDB
}

func (s *managedDatabaseStore) GetByID(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
	mdb := new(model.ManagedDatabase)
	err := s.db.NewSelect().Model(mdb).Where("id = ?", id).Scan(ctx)
	return mdb, err
}

func (s *managedDatabaseStore) Create(ctx context.Context, db *model.ManagedDatabase) error {
	_, err := s.db.NewInsert().Model(db).Returning("*").Exec(ctx)
	return err
}

func (s *managedDatabaseStore) Update(ctx context.Context, db *model.ManagedDatabase) error {
	_, err := s.db.NewUpdate().Model(db).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *managedDatabaseStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.ManagedDatabase)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *managedDatabaseStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.ManagedDatabase, int, error) {
	var dbs []model.ManagedDatabase
	count, err := s.db.NewSelect().
		Model(&dbs).
		Where("project_id = ?", projectID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return dbs, count, err
}

func (s *managedDatabaseStore) ListAll(ctx context.Context, params store.ListParams, filter store.DatabaseListFilter) ([]model.ManagedDatabase, int, error) {
	var dbs []model.ManagedDatabase
	if filter.ProjectIDs != nil && len(filter.ProjectIDs) == 0 {
		return dbs, 0, nil
	}
	q := s.db.NewSelect().Model(&dbs)

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
	return dbs, count, err
}

func (s *managedDatabaseStore) ExistsByK8sName(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error) {
	return s.db.NewSelect().
		Model((*model.ManagedDatabase)(nil)).
		Where("project_id = ?", projectID).
		Where("k8s_name = ?", k8sName).
		Exists(ctx)
}

func (s *managedDatabaseStore) FindByBackupS3(ctx context.Context, s3ID uuid.UUID) (*model.ManagedDatabase, error) {
	mdb := new(model.ManagedDatabase)
	err := s.db.NewSelect().
		Model(mdb).
		Where("backup_s3_id = ?", s3ID).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mdb, nil
}

func (s *managedDatabaseStore) FindByExternalPort(ctx context.Context, port int32) (*model.ManagedDatabase, error) {
	mdb := new(model.ManagedDatabase)
	err := s.db.NewSelect().Model(mdb).
		Where("external_enabled = true").
		Where("external_port = ?", port).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mdb, nil
}

func (s *managedDatabaseStore) ListBackupEnabled(ctx context.Context) ([]model.ManagedDatabase, error) {
	var dbs []model.ManagedDatabase
	err := s.db.NewSelect().
		Model(&dbs).
		Where("backup_enabled = true").
		Where("backup_schedule != ''").
		Scan(ctx)
	return dbs, err
}

func (s *managedDatabaseStore) ListExternalPorts(ctx context.Context) ([]model.ExternalPortInfo, error) {
	var result []model.ExternalPortInfo
	err := s.db.NewSelect().
		TableExpr("managed_databases").
		ColumnExpr("id AS database_id").
		ColumnExpr("name AS database_name").
		ColumnExpr("engine").
		ColumnExpr("external_port AS port").
		Where("external_enabled = true").
		Where("external_port > 0").
		OrderExpr("external_port ASC").
		Scan(ctx, &result)
	return result, err
}
