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

type pageStore struct {
	db      bun.IDB
	secrets secret.Store
}

func (s *pageStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Page, error) {
	page := new(model.Page)
	err := s.db.NewSelect().Model(page).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptPage(page, s.secrets); err != nil {
		return nil, err
	}
	return page, nil
}

func (s *pageStore) Create(ctx context.Context, page *model.Page) error {
	if err := encryptPageWebhook(s.secrets, page); err != nil {
		return err
	}
	_, err := s.db.NewInsert().Model(page).Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptPage(page, s.secrets)
}

func (s *pageStore) Update(ctx context.Context, page *model.Page) error {
	if err := encryptPageWebhook(s.secrets, page); err != nil {
		return err
	}
	_, err := s.db.NewUpdate().Model(page).WherePK().Returning("*").Exec(ctx)
	if err != nil {
		return err
	}
	return decryptPage(page, s.secrets)
}

// UpdateSettings writes only the user-editable settings columns, deliberately
// excluding "runtime" and "status". The worker persists those incrementally
// during a deploy (e.g. BucketName/DistributionID/OACID over CloudFront's
// ~10-minute provisioning window); a full-row save from a concurrent settings
// PATCH would clobber that runtime state and orphan the provisioned AWS
// resources. Returning("*") refreshes the page with the current runtime/status
// from the DB so the caller never echoes a stale snapshot.
func (s *pageStore) UpdateSettings(ctx context.Context, page *model.Page) error {
	_, err := s.db.NewUpdate().
		Model(page).
		Column("description", "git_repo", "git_branch", "git_provider_id", "publish_path", "cloud_account_id", "region", "custom_domain", "manage_dns", "dns_account_id", "dns_zone_id", "build_enabled", "package_manager", "install_command", "build_command", "output_dir", "root_directory", "node_version", "build_env_vars").
		Set("updated_at = current_timestamp").
		WherePK().
		Returning("*").
		Exec(ctx)
	return err
}

// UpdateSettingsIfNotDeploying writes the same settings columns as
// UpdateSettings but only when the row is not mid-deploy, atomically. Returns
// false when no row matched (the page is deploying), so the caller can reject a
// cloud-target change that raced a concurrent TryMarkDeploying.
func (s *pageStore) UpdateSettingsIfNotDeploying(ctx context.Context, page *model.Page) (bool, error) {
	res, err := s.db.NewUpdate().
		Model(page).
		Column("description", "git_repo", "git_branch", "git_provider_id", "publish_path", "cloud_account_id", "region", "custom_domain", "manage_dns", "dns_account_id", "dns_zone_id", "build_enabled", "package_manager", "install_command", "build_command", "output_dir", "root_directory", "node_version", "build_env_vars").
		Set("updated_at = current_timestamp").
		WherePK().
		Where("status <> ?", model.PageStatusDeploying).
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

func (s *pageStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Page)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

// DeleteIfNotDeploying deletes the page only when it isn't mid-deploy and
// returns the deleted row (for cloud teardown), or nil if nothing was deleted.
// The atomic "status <> deploying" guard serializes against TryMarkDeploying:
// once this removes the row a deploy can't start, and if a deploy is already
// running this matches 0 rows and the caller refuses without touching AWS.
func (s *pageStore) DeleteIfNotDeploying(ctx context.Context, id uuid.UUID) (*model.Page, error) {
	page := new(model.Page)
	res, err := s.db.NewDelete().
		Model(page).
		Where("id = ?", id).
		Where("status <> ?", model.PageStatusDeploying).
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
	if err := decryptPage(page, s.secrets); err != nil {
		return nil, err
	}
	return page, nil
}

func (s *pageStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Page, int, error) {
	var pages []model.Page
	count, err := s.db.NewSelect().
		Model(&pages).
		Where("project_id = ?", projectID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := decryptPages(pages, s.secrets); err != nil {
		return nil, 0, err
	}
	return pages, count, nil
}

func (s *pageStore) ListAll(ctx context.Context, params store.ListParams, filter store.PageListFilter) ([]model.Page, int, error) {
	var pages []model.Page
	if filter.ProjectIDs != nil && len(filter.ProjectIDs) == 0 {
		return pages, 0, nil
	}
	q := s.db.NewSelect().Model(&pages)

	if filter.Search != "" {
		q = q.Where("LOWER(name) LIKE LOWER(?) ESCAPE '\\'", likeContains(filter.Search))
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Provider != "" {
		q = q.Where("provider = ?", filter.Provider)
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
	if err := decryptPages(pages, s.secrets); err != nil {
		return nil, 0, err
	}
	return pages, count, nil
}

func (s *pageStore) ExistsByName(ctx context.Context, projectID uuid.UUID, name string) (bool, error) {
	return s.db.NewSelect().
		Model((*model.Page)(nil)).
		Where("project_id = ?", projectID).
		Where("name = ?", name).
		Exists(ctx)
}

func (s *pageStore) FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Page, error) {
	page := new(model.Page)
	// Only the columns the delete-guard inspects; avoids decrypting webhook_secret.
	err := s.db.NewSelect().
		Model(page).
		Column("id", "name", "cloud_account_id", "git_provider_id", "dns_account_id").
		Where("cloud_account_id = ? OR git_provider_id = ? OR dns_account_id = ?", resourceID, resourceID, resourceID).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (s *pageStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.PageStatus) error {
	_, err := s.db.NewUpdate().
		Model((*model.Page)(nil)).
		Set("status = ?", status).
		Set("updated_at = current_timestamp").
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *pageStore) UpdateRuntime(ctx context.Context, id uuid.UUID, rt *model.PageRuntime) error {
	page := &model.Page{Runtime: rt}
	page.ID = id
	_, err := s.db.NewUpdate().
		Model(page).
		Column("runtime").
		Set("updated_at = current_timestamp").
		WherePK().
		Exec(ctx)
	return err
}

func (s *pageStore) TryMarkDeploying(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := s.db.NewUpdate().
		Model((*model.Page)(nil)).
		Set("status = ?", model.PageStatusDeploying).
		Set("updated_at = current_timestamp").
		Where("id = ?", id).
		Where("status <> ?", model.PageStatusDeploying).
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

type pageDeploymentStore struct {
	db bun.IDB
}

func (s *pageDeploymentStore) GetByID(ctx context.Context, id uuid.UUID) (*model.PageDeployment, error) {
	dep := new(model.PageDeployment)
	err := s.db.NewSelect().Model(dep).Where("id = ?", id).Scan(ctx)
	return dep, err
}

func (s *pageDeploymentStore) Create(ctx context.Context, dep *model.PageDeployment) error {
	_, err := s.db.NewInsert().Model(dep).Returning("*").Exec(ctx)
	return err
}

func (s *pageDeploymentStore) Update(ctx context.Context, dep *model.PageDeployment) error {
	_, err := s.db.NewUpdate().Model(dep).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *pageDeploymentStore) ListByPage(ctx context.Context, pageID uuid.UUID, params store.ListParams) ([]model.PageDeployment, int, error) {
	var deps []model.PageDeployment
	count, err := s.db.NewSelect().
		Model(&deps).
		Where("page_id = ?", pageID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deps, count, err
}

func (s *pageDeploymentStore) GetLatestByPage(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error) {
	dep := new(model.PageDeployment)
	err := s.db.NewSelect().
		Model(dep).
		Where("page_id = ?", pageID).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	return dep, err
}

func (s *pageDeploymentStore) GetLatestSuccess(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error) {
	dep := new(model.PageDeployment)
	err := s.db.NewSelect().
		Model(dep).
		Where("page_id = ?", pageID).
		Where("status = ?", model.PageDeploySuccess).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	return dep, err
}

func (s *pageDeploymentStore) MarkTimedOut(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error) {
	res, err := s.db.NewUpdate().
		Model((*model.PageDeployment)(nil)).
		Set("status = ?", model.PageDeployFailed).
		Set("finished_at = ?", finishedAt).
		Set("deploy_log = deploy_log || ?", logSuffix).
		Set("updated_at = current_timestamp").
		Where("id = ?", id).
		Where("status = ?", model.PageDeployDeploying).
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

func (s *pageDeploymentStore) ListByStatus(ctx context.Context, status model.PageDeploymentStatus, params store.ListParams) ([]model.PageDeployment, int, error) {
	var deps []model.PageDeployment
	count, err := s.db.NewSelect().
		Model(&deps).
		Where("status = ?", status).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deps, count, err
}
