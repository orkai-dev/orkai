package testsupport

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// ─── WorkerStore ───────────────────────────────────────────

type FakeWorkerStore struct {
	GetByIDFn                      func(ctx context.Context, id uuid.UUID) (*model.Worker, error)
	CreateFn                       func(ctx context.Context, worker *model.Worker) error
	UpdateFn                       func(ctx context.Context, worker *model.Worker) error
	UpdateSettingsFn               func(ctx context.Context, worker *model.Worker) error
	UpdateSettingsIfNotDeployingFn func(ctx context.Context, worker *model.Worker) (bool, error)
	DeleteFn                       func(ctx context.Context, id uuid.UUID) error
	DeleteIfNotDeployingFn         func(ctx context.Context, id uuid.UUID) (*model.Worker, error)
	ListByProjectFn                func(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Worker, int, error)
	ListAllFn                      func(ctx context.Context, params store.ListParams, filter store.WorkerListFilter) ([]model.Worker, int, error)
	ExistsByNameFn                 func(ctx context.Context, projectID uuid.UUID, name string) (bool, error)
	FindByResourceFn               func(ctx context.Context, resourceID uuid.UUID) (*model.Worker, error)
	TryMarkDeployingFn             func(ctx context.Context, id uuid.UUID) (bool, error)
	UpdateStatusFn                 func(ctx context.Context, id uuid.UUID, status model.WorkerStatus) error
	UpdateRuntimeFn                func(ctx context.Context, id uuid.UUID, rt *model.WorkerRuntime) error

	records map[uuid.UUID]*model.Worker
}

func (f *FakeWorkerStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Worker, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	if rec, ok := f.records[id]; ok {
		return rec, nil
	}
	return nil, errors.New("not found")
}

func (f *FakeWorkerStore) Create(ctx context.Context, worker *model.Worker) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, worker)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.Worker{}
	}
	if worker.ID == uuid.Nil {
		worker.ID = uuid.New()
	}
	f.records[worker.ID] = worker
	return nil
}

func (f *FakeWorkerStore) Update(ctx context.Context, worker *model.Worker) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, worker)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.Worker{}
	}
	f.records[worker.ID] = worker
	return nil
}

func (f *FakeWorkerStore) UpdateSettings(ctx context.Context, worker *model.Worker) error {
	if f.UpdateSettingsFn != nil {
		return f.UpdateSettingsFn(ctx, worker)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.Worker{}
	}
	rec, ok := f.records[worker.ID]
	if !ok {
		return errors.New("not found")
	}
	applyWorkerSettings(rec, worker)
	worker.Runtime = rec.Runtime
	worker.Status = rec.Status
	return nil
}

func (f *FakeWorkerStore) UpdateSettingsIfNotDeploying(ctx context.Context, worker *model.Worker) (bool, error) {
	if f.UpdateSettingsIfNotDeployingFn != nil {
		return f.UpdateSettingsIfNotDeployingFn(ctx, worker)
	}
	rec, ok := f.records[worker.ID]
	if !ok {
		return false, errors.New("not found")
	}
	if rec.Status == model.WorkerStatusDeploying {
		return false, nil
	}
	applyWorkerSettings(rec, worker)
	worker.Runtime = rec.Runtime
	worker.Status = rec.Status
	return true, nil
}

func applyWorkerSettings(rec, worker *model.Worker) {
	rec.Description = worker.Description
	rec.GitRepo = worker.GitRepo
	rec.GitBranch = worker.GitBranch
	rec.GitProviderID = worker.GitProviderID
	rec.RootDirectory = worker.RootDirectory
	rec.WranglerConfig = worker.WranglerConfig
	rec.PackageManager = worker.PackageManager
	rec.InstallCommand = worker.InstallCommand
	rec.BuildCommand = worker.BuildCommand
	rec.DeployCommand = worker.DeployCommand
	rec.BuildEnvVars = worker.BuildEnvVars
	rec.CloudAccountID = worker.CloudAccountID
}

func (f *FakeWorkerStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	delete(f.records, id)
	return nil
}

func (f *FakeWorkerStore) DeleteIfNotDeploying(ctx context.Context, id uuid.UUID) (*model.Worker, error) {
	if f.DeleteIfNotDeployingFn != nil {
		return f.DeleteIfNotDeployingFn(ctx, id)
	}
	rec, ok := f.records[id]
	if !ok {
		return nil, nil
	}
	if rec.Status == model.WorkerStatusDeploying {
		return nil, nil
	}
	delete(f.records, id)
	return rec, nil
}

func (f *FakeWorkerStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Worker, int, error) {
	if f.ListByProjectFn != nil {
		return f.ListByProjectFn(ctx, projectID, params)
	}
	var out []model.Worker
	for _, r := range f.records {
		if r.ProjectID == projectID {
			out = append(out, *r)
		}
	}
	return out, len(out), nil
}

func (f *FakeWorkerStore) ListAll(ctx context.Context, params store.ListParams, filter store.WorkerListFilter) ([]model.Worker, int, error) {
	if f.ListAllFn != nil {
		return f.ListAllFn(ctx, params, filter)
	}
	var out []model.Worker
	for _, r := range f.records {
		out = append(out, *r)
	}
	return out, len(out), nil
}

func (f *FakeWorkerStore) ExistsByName(ctx context.Context, projectID uuid.UUID, name string) (bool, error) {
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

func (f *FakeWorkerStore) FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Worker, error) {
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

func (f *FakeWorkerStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.WorkerStatus) error {
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

func (f *FakeWorkerStore) UpdateRuntime(ctx context.Context, id uuid.UUID, rt *model.WorkerRuntime) error {
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

func (f *FakeWorkerStore) TryMarkDeploying(ctx context.Context, id uuid.UUID) (bool, error) {
	if f.TryMarkDeployingFn != nil {
		return f.TryMarkDeployingFn(ctx, id)
	}
	rec, ok := f.records[id]
	if !ok {
		return false, errors.New("not found")
	}
	if rec.Status == model.WorkerStatusDeploying {
		return false, nil
	}
	rec.Status = model.WorkerStatusDeploying
	return true, nil
}

// ─── WorkerDeploymentStore ───────────────────────────────────────────

type FakeWorkerDeploymentStore struct {
	GetByIDFn                   func(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error)
	CreateFn                    func(ctx context.Context, dep *model.WorkerDeployment) error
	UpdateFn                    func(ctx context.Context, dep *model.WorkerDeployment) error
	ListByWorkerFn              func(ctx context.Context, workerID uuid.UUID, params store.ListParams) ([]model.WorkerDeployment, int, error)
	GetLatestByWorkerFn         func(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error)
	GetLatestSuccessFn          func(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error)
	ListByStatusFn              func(ctx context.Context, status model.WorkerDeploymentStatus, params store.ListParams) ([]model.WorkerDeployment, int, error)
	MarkTimedOutFn              func(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error)
	TryClaimNeedsConfirmationFn func(ctx context.Context, id uuid.UUID) (bool, error)

	records map[uuid.UUID]*model.WorkerDeployment
}

func (f *FakeWorkerDeploymentStore) GetByID(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	if rec, ok := f.records[id]; ok {
		return rec, nil
	}
	return nil, errors.New("not found")
}

func (f *FakeWorkerDeploymentStore) Create(ctx context.Context, dep *model.WorkerDeployment) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, dep)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.WorkerDeployment{}
	}
	if dep.ID == uuid.Nil {
		dep.ID = uuid.New()
	}
	f.records[dep.ID] = dep
	return nil
}

func (f *FakeWorkerDeploymentStore) Update(ctx context.Context, dep *model.WorkerDeployment) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, dep)
	}
	if f.records == nil {
		f.records = map[uuid.UUID]*model.WorkerDeployment{}
	}
	f.records[dep.ID] = dep
	return nil
}

func (f *FakeWorkerDeploymentStore) ListByWorker(ctx context.Context, workerID uuid.UUID, params store.ListParams) ([]model.WorkerDeployment, int, error) {
	if f.ListByWorkerFn != nil {
		return f.ListByWorkerFn(ctx, workerID, params)
	}
	var out []model.WorkerDeployment
	for _, r := range f.records {
		if r.WorkerID == workerID {
			out = append(out, *r)
		}
	}
	return out, len(out), nil
}

func (f *FakeWorkerDeploymentStore) GetLatestByWorker(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error) {
	if f.GetLatestByWorkerFn != nil {
		return f.GetLatestByWorkerFn(ctx, workerID)
	}
	return nil, errors.New("not found")
}

func (f *FakeWorkerDeploymentStore) GetLatestSuccess(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error) {
	if f.GetLatestSuccessFn != nil {
		return f.GetLatestSuccessFn(ctx, workerID)
	}
	return nil, errors.New("not found")
}

func (f *FakeWorkerDeploymentStore) MarkTimedOut(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error) {
	if f.MarkTimedOutFn != nil {
		return f.MarkTimedOutFn(ctx, id, finishedAt, logSuffix)
	}
	rec, ok := f.records[id]
	if !ok {
		return false, errors.New("not found")
	}
	if rec.Status != model.WorkerDeployDeploying {
		return false, nil
	}
	rec.Status = model.WorkerDeployFailed
	rec.FinishedAt = &finishedAt
	rec.DeployLog += logSuffix
	return true, nil
}

func (f *FakeWorkerDeploymentStore) TryClaimNeedsConfirmation(ctx context.Context, id uuid.UUID) (bool, error) {
	if f.TryClaimNeedsConfirmationFn != nil {
		return f.TryClaimNeedsConfirmationFn(ctx, id)
	}
	rec, ok := f.records[id]
	if !ok {
		return false, errors.New("not found")
	}
	if rec.Status != model.WorkerDeployNeedsConfirmation {
		return false, nil
	}
	rec.Status = model.WorkerDeployCancelled
	rec.DeployLog += "\n[orkai] R2 confirmation in progress…"
	return true, nil
}

func (f *FakeWorkerDeploymentStore) ListByStatus(ctx context.Context, status model.WorkerDeploymentStatus, params store.ListParams) ([]model.WorkerDeployment, int, error) {
	if f.ListByStatusFn != nil {
		return f.ListByStatusFn(ctx, status, params)
	}
	var out []model.WorkerDeployment
	for _, r := range f.records {
		if r.Status == status {
			out = append(out, *r)
		}
	}
	return out, len(out), nil
}
