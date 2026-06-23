package pg

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type workerStore struct {
	db      bun.IDB
	secrets secret.Store
}

// workerSettingsColumns are the user-editable settings columns, deliberately
// excluding "runtime" and "status" which the deploy worker persists.
var workerSettingsColumns = []string{
	"description", "git_repo", "git_branch", "git_provider_id",
	"root_directory", "wrangler_config", "package_manager", "install_command",
	"build_command", "deploy_command", "build_env_vars", "cloud_account_id",
}

func (s *workerStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Worker, error) {
	worker := new(model.Worker)
	err := s.db.NewSelect().Model(worker).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptWorker(worker, s.secrets); err != nil {
		return nil, err
	}
	return worker, nil
}

func (s *workerStore) Create(ctx context.Context, worker *model.Worker) error {
	if err := encryptWorkerWebhook(s.secrets, worker); err != nil {
		return err
	}
	_, err := s.db.NewInsert().Model(worker).Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptWorker(worker, s.secrets)
}

func (s *workerStore) Update(ctx context.Context, worker *model.Worker) error {
	if err := encryptWorkerWebhook(s.secrets, worker); err != nil {
		return err
	}
	_, err := s.db.NewUpdate().Model(worker).WherePK().Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptWorker(worker, s.secrets)
}

func (s *workerStore) UpdateSettings(ctx context.Context, worker *model.Worker) error {
	_, err := s.db.NewUpdate().
		Model(worker).
		Column(workerSettingsColumns...).
		Set("updated_at = current_timestamp").
		WherePK().
		Returning("*").
		Exec(ctx)
	return err
}

func (s *workerStore) UpdateSettingsIfNotDeploying(ctx context.Context, worker *model.Worker) (bool, error) {
	res, err := s.db.NewUpdate().
		Model(worker).
		Column(workerSettingsColumns...).
		Set("updated_at = current_timestamp").
		WherePK().
		Where("status <> ?", model.WorkerStatusDeploying).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *workerStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Worker)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *workerStore) DeleteIfNotDeploying(ctx context.Context, id uuid.UUID) (*model.Worker, error) {
	worker := new(model.Worker)
	res, err := s.db.NewDelete().
		Model(worker).
		Where("id = ?", id).
		Where("status <> ?", model.WorkerStatusDeploying).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	if err := decryptWorker(worker, s.secrets); err != nil {
		return nil, err
	}
	return worker, nil
}

func (s *workerStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Worker, int, error) {
	var workers []model.Worker
	count, err := s.db.NewSelect().
		Model(&workers).
		Where("project_id = ?", projectID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := decryptWorkers(workers, s.secrets); err != nil {
		return nil, 0, err
	}
	return workers, count, nil
}

func (s *workerStore) ListAll(ctx context.Context, params store.ListParams, filter store.WorkerListFilter) ([]model.Worker, int, error) {
	var workers []model.Worker
	if filter.ProjectIDs != nil && len(filter.ProjectIDs) == 0 {
		return workers, 0, nil
	}
	q := s.db.NewSelect().Model(&workers)

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
	if err := decryptWorkers(workers, s.secrets); err != nil {
		return nil, 0, err
	}
	return workers, count, nil
}

func (s *workerStore) ExistsByName(ctx context.Context, projectID uuid.UUID, name string) (bool, error) {
	return s.db.NewSelect().
		Model((*model.Worker)(nil)).
		Where("project_id = ?", projectID).
		Where("name = ?", name).
		Exists(ctx)
}

func (s *workerStore) FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Worker, error) {
	worker := new(model.Worker)
	err := s.db.NewSelect().
		Model(worker).
		Column("id", "name", "cloud_account_id", "git_provider_id").
		Where("cloud_account_id = ? OR git_provider_id = ?", resourceID, resourceID).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return worker, nil
}

func (s *workerStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.WorkerStatus) error {
	_, err := s.db.NewUpdate().
		Model((*model.Worker)(nil)).
		Set("status = ?", status).
		Set("updated_at = current_timestamp").
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *workerStore) UpdateRuntime(ctx context.Context, id uuid.UUID, rt *model.WorkerRuntime) error {
	worker := &model.Worker{Runtime: rt}
	worker.ID = id
	_, err := s.db.NewUpdate().
		Model(worker).
		Column("runtime").
		Set("updated_at = current_timestamp").
		WherePK().
		Exec(ctx)
	return err
}

func (s *workerStore) TryMarkDeploying(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := s.db.NewUpdate().
		Model((*model.Worker)(nil)).
		Set("status = ?", model.WorkerStatusDeploying).
		Set("updated_at = current_timestamp").
		Where("id = ?", id).
		Where("status <> ?", model.WorkerStatusDeploying).
		Exec(ctx)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

type workerDeploymentStore struct {
	db bun.IDB
}

func (s *workerDeploymentStore) GetByID(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error) {
	dep := new(model.WorkerDeployment)
	err := s.db.NewSelect().Model(dep).Where("id = ?", id).Scan(ctx)
	return dep, err
}

func (s *workerDeploymentStore) Create(ctx context.Context, dep *model.WorkerDeployment) error {
	_, err := s.db.NewInsert().Model(dep).Returning("*").Exec(ctx)
	return err
}

func (s *workerDeploymentStore) Update(ctx context.Context, dep *model.WorkerDeployment) error {
	_, err := s.db.NewUpdate().Model(dep).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *workerDeploymentStore) ListByWorker(ctx context.Context, workerID uuid.UUID, params store.ListParams) ([]model.WorkerDeployment, int, error) {
	var deps []model.WorkerDeployment
	count, err := s.db.NewSelect().
		Model(&deps).
		Where("worker_id = ?", workerID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deps, count, err
}

func (s *workerDeploymentStore) GetLatestByWorker(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error) {
	dep := new(model.WorkerDeployment)
	err := s.db.NewSelect().
		Model(dep).
		Where("worker_id = ?", workerID).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	return dep, err
}

func (s *workerDeploymentStore) GetLatestSuccess(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error) {
	dep := new(model.WorkerDeployment)
	err := s.db.NewSelect().
		Model(dep).
		Where("worker_id = ?", workerID).
		Where("status = ?", model.WorkerDeploySuccess).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	return dep, err
}

func (s *workerDeploymentStore) MarkTimedOut(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error) {
	res, err := s.db.NewUpdate().
		Model((*model.WorkerDeployment)(nil)).
		Set("status = ?", model.WorkerDeployFailed).
		Set("finished_at = ?", finishedAt).
		Set("deploy_log = deploy_log || ?", logSuffix).
		Set("updated_at = current_timestamp").
		Where("id = ?", id).
		Where("status = ?", model.WorkerDeployDeploying).
		Exec(ctx)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *workerDeploymentStore) TryClaimNeedsConfirmation(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := s.db.NewUpdate().
		Model((*model.WorkerDeployment)(nil)).
		Set("status = ?", model.WorkerDeployCancelled).
		Set("deploy_log = deploy_log || ?", "\n[orkai] R2 confirmation in progress…").
		Set("updated_at = current_timestamp").
		Where("id = ?", id).
		Where("status = ?", model.WorkerDeployNeedsConfirmation).
		Exec(ctx)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *workerDeploymentStore) ListByStatus(ctx context.Context, status model.WorkerDeploymentStatus, params store.ListParams) ([]model.WorkerDeployment, int, error) {
	var deps []model.WorkerDeployment
	count, err := s.db.NewSelect().
		Model(&deps).
		Where("status = ?", status).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deps, count, err
}
