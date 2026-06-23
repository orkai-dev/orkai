package testsupport

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// ─── PageStore ───────────────────────────────────────────

type FakePageStore struct {
	GetByIDFn                      func(ctx context.Context, id uuid.UUID) (*model.Page, error)
	CreateFn                       func(ctx context.Context, page *model.Page) error
	UpdateFn                       func(ctx context.Context, page *model.Page) error
	UpdateSettingsFn               func(ctx context.Context, page *model.Page) error
	UpdateSettingsIfNotDeployingFn func(ctx context.Context, page *model.Page) (bool, error)
	DeleteFn                       func(ctx context.Context, id uuid.UUID) error
	DeleteIfNotDeployingFn         func(ctx context.Context, id uuid.UUID) (*model.Page, error)
	ListByProjectFn                func(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Page, int, error)
	ListAllFn                      func(ctx context.Context, params store.ListParams, filter store.PageListFilter) ([]model.Page, int, error)
	ExistsByNameFn                 func(ctx context.Context, projectID uuid.UUID, name string) (bool, error)
	FindByResourceFn               func(ctx context.Context, resourceID uuid.UUID) (*model.Page, error)
	TryMarkDeployingFn             func(ctx context.Context, id uuid.UUID) (bool, error)
	UpdateStatusFn                 func(ctx context.Context, id uuid.UUID, status model.PageStatus) error
	UpdateRuntimeFn                func(ctx context.Context, id uuid.UUID, rt *model.PageRuntime) error

	records map[uuid.UUID]*model.Page
}

func (f *FakePageStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Page, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	if rec, ok := f.records[id]; ok {
		return rec, nil
	}
	return nil, errors.New("not found")
}

func (f *FakePageStore) Create(ctx context.Context, page *model.Page) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, page)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.Page{}
	}
	if page.ID == uuid.Nil {
		page.ID = uuid.New()
	}
	f.records[page.ID] = page
	return nil
}

func (f *FakePageStore) Update(ctx context.Context, page *model.Page) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, page)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.Page{}
	}
	f.records[page.ID] = page
	return nil
}

// UpdateSettings mirrors the pg store: it persists only the editable settings
// columns and preserves the stored runtime and status so concurrent worker
// writes are not clobbered.
func (f *FakePageStore) UpdateSettings(ctx context.Context, page *model.Page) error {
	if f.UpdateSettingsFn != nil {
		return f.UpdateSettingsFn(ctx, page)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.Page{}
	}
	rec, ok := f.records[page.ID]
	if !ok {
		return errors.New("not found")
	}
	rec.Description = page.Description
	rec.GitRepo = page.GitRepo
	rec.GitBranch = page.GitBranch
	rec.GitProviderID = page.GitProviderID
	rec.PublishPath = page.PublishPath
	rec.CloudAccountID = page.CloudAccountID
	rec.Region = page.Region
	// Reflect the persisted runtime/status back onto the caller's struct, like
	// Returning("*") does in the real store.
	page.Runtime = rec.Runtime
	page.Status = rec.Status
	return nil
}

// UpdateSettingsIfNotDeploying mirrors the pg store: it writes only the editable
// settings columns and only when the page is not mid-deploy, returning false
// (without writing) when the page is deploying.
func (f *FakePageStore) UpdateSettingsIfNotDeploying(ctx context.Context, page *model.Page) (bool, error) {
	if f.UpdateSettingsIfNotDeployingFn != nil {
		return f.UpdateSettingsIfNotDeployingFn(ctx, page)
	}
	rec, ok := f.records[page.ID]
	if !ok {
		return false, errors.New("not found")
	}
	if rec.Status == model.PageStatusDeploying {
		return false, nil
	}
	rec.Description = page.Description
	rec.GitRepo = page.GitRepo
	rec.GitBranch = page.GitBranch
	rec.GitProviderID = page.GitProviderID
	rec.PublishPath = page.PublishPath
	rec.CloudAccountID = page.CloudAccountID
	rec.Region = page.Region
	page.Runtime = rec.Runtime
	page.Status = rec.Status
	return true, nil
}

func (f *FakePageStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	delete(f.records, id)
	return nil
}

// DeleteIfNotDeploying mirrors the pg store: it removes the page only when it is
// not mid-deploy and returns the deleted record (nil if a deploy is running).
func (f *FakePageStore) DeleteIfNotDeploying(ctx context.Context, id uuid.UUID) (*model.Page, error) {
	if f.DeleteIfNotDeployingFn != nil {
		return f.DeleteIfNotDeployingFn(ctx, id)
	}
	rec, ok := f.records[id]
	if !ok {
		return nil, nil
	}
	if rec.Status == model.PageStatusDeploying {
		return nil, nil
	}
	delete(f.records, id)
	return rec, nil
}

func (f *FakePageStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Page, int, error) {
	if f.ListByProjectFn != nil {
		return f.ListByProjectFn(ctx, projectID, params)
	}
	var out []model.Page
	for _, r := range f.records {
		if r.ProjectID == projectID {
			out = append(out, *r)
		}
	}
	return out, len(out), nil
}

func (f *FakePageStore) ListAll(ctx context.Context, params store.ListParams, filter store.PageListFilter) ([]model.Page, int, error) {
	if f.ListAllFn != nil {
		return f.ListAllFn(ctx, params, filter)
	}
	var out []model.Page
	for _, r := range f.records {
		out = append(out, *r)
	}
	return out, len(out), nil
}

func (f *FakePageStore) ExistsByName(ctx context.Context, projectID uuid.UUID, name string) (bool, error) {
	if f.ExistsByNameFn != nil {
		return f.ExistsByNameFn(ctx, projectID, name)
	}
	for _, r := range f.records {
		if r.ProjectID == projectID && r.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (f *FakePageStore) FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Page, error) {
	if f.FindByResourceFn != nil {
		return f.FindByResourceFn(ctx, resourceID)
	}
	for _, r := range f.records {
		if (r.CloudAccountID != nil && *r.CloudAccountID == resourceID) ||
			(r.GitProviderID != nil && *r.GitProviderID == resourceID) {
			return r, nil
		}
	}
	return nil, nil
}

func (f *FakePageStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.PageStatus) error {
	if f.UpdateStatusFn != nil {
		return f.UpdateStatusFn(ctx, id, status)
	}
	rec, ok := f.records[id]
	if !ok {
		return errors.New("not found")
	}
	rec.Status = status
	return nil
}

func (f *FakePageStore) UpdateRuntime(ctx context.Context, id uuid.UUID, rt *model.PageRuntime) error {
	if f.UpdateRuntimeFn != nil {
		return f.UpdateRuntimeFn(ctx, id, rt)
	}
	rec, ok := f.records[id]
	if !ok {
		return errors.New("not found")
	}
	rec.Runtime = rt
	return nil
}

func (f *FakePageStore) TryMarkDeploying(ctx context.Context, id uuid.UUID) (bool, error) {
	if f.TryMarkDeployingFn != nil {
		return f.TryMarkDeployingFn(ctx, id)
	}
	rec, ok := f.records[id]
	if !ok {
		return false, errors.New("not found")
	}
	if rec.Status == model.PageStatusDeploying {
		return false, nil
	}
	rec.Status = model.PageStatusDeploying
	return true, nil
}

// ─── PageDeploymentStore ───────────────────────────────────────────

type FakePageDeploymentStore struct {
	GetByIDFn          func(ctx context.Context, id uuid.UUID) (*model.PageDeployment, error)
	CreateFn           func(ctx context.Context, dep *model.PageDeployment) error
	UpdateFn           func(ctx context.Context, dep *model.PageDeployment) error
	ListByPageFn       func(ctx context.Context, pageID uuid.UUID, params store.ListParams) ([]model.PageDeployment, int, error)
	GetLatestByPageFn  func(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error)
	GetLatestSuccessFn func(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error)
	ListByStatusFn     func(ctx context.Context, status model.PageDeploymentStatus, params store.ListParams) ([]model.PageDeployment, int, error)
	MarkTimedOutFn     func(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error)

	records map[uuid.UUID]*model.PageDeployment
}

func (f *FakePageDeploymentStore) GetByID(ctx context.Context, id uuid.UUID) (*model.PageDeployment, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	if rec, ok := f.records[id]; ok {
		return rec, nil
	}
	return nil, errors.New("not found")
}

func (f *FakePageDeploymentStore) Create(ctx context.Context, dep *model.PageDeployment) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, dep)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.PageDeployment{}
	}
	if dep.ID == uuid.Nil {
		dep.ID = uuid.New()
	}
	f.records[dep.ID] = dep
	return nil
}

func (f *FakePageDeploymentStore) Update(ctx context.Context, dep *model.PageDeployment) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, dep)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.PageDeployment{}
	}
	f.records[dep.ID] = dep
	return nil
}

func (f *FakePageDeploymentStore) ListByPage(ctx context.Context, pageID uuid.UUID, params store.ListParams) ([]model.PageDeployment, int, error) {
	if f.ListByPageFn != nil {
		return f.ListByPageFn(ctx, pageID, params)
	}
	var out []model.PageDeployment
	for _, r := range f.records {
		if r.PageID == pageID {
			out = append(out, *r)
		}
	}
	return out, len(out), nil
}

func (f *FakePageDeploymentStore) GetLatestByPage(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error) {
	if f.GetLatestByPageFn != nil {
		return f.GetLatestByPageFn(ctx, pageID)
	}
	return nil, errors.New("not found")
}

func (f *FakePageDeploymentStore) GetLatestSuccess(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error) {
	if f.GetLatestSuccessFn != nil {
		return f.GetLatestSuccessFn(ctx, pageID)
	}
	return nil, errors.New("not found")
}

// MarkTimedOut mirrors the pg store: it flips the deployment to failed only
// while it is still deploying, returning false otherwise.
func (f *FakePageDeploymentStore) MarkTimedOut(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error) {
	if f.MarkTimedOutFn != nil {
		return f.MarkTimedOutFn(ctx, id, finishedAt, logSuffix)
	}
	rec, ok := f.records[id]
	if !ok {
		return false, errors.New("not found")
	}
	if rec.Status != model.PageDeployDeploying {
		return false, nil
	}
	rec.Status = model.PageDeployFailed
	rec.FinishedAt = &finishedAt
	rec.DeployLog += logSuffix
	return true, nil
}

func (f *FakePageDeploymentStore) ListByStatus(ctx context.Context, status model.PageDeploymentStatus, params store.ListParams) ([]model.PageDeployment, int, error) {
	if f.ListByStatusFn != nil {
		return f.ListByStatusFn(ctx, status, params)
	}
	var out []model.PageDeployment
	for _, r := range f.records {
		if r.Status == status {
			out = append(out, *r)
		}
	}
	return out, len(out), nil
}
