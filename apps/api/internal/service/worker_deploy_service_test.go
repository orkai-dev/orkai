package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
)

func newWorkerDeployService(fs *testsupport.FakeStore, queue *testsupport.FakeEnqueuer) *WorkerDeployService {
	return NewWorkerDeployService(
		fs,
		nil,
		queue,
		testsupport.NewFakeTargetRegistry(testsupport.NewFakeOrchestrator()),
		testsupport.NewProviders(fs),
		testsupport.NewTestLogger(),
	)
}

func seedWorkerConfirmFixture(t *testing.T) (*testsupport.FakeStore, *testsupport.FakeEnqueuer, uuid.UUID, *model.Worker, *model.WorkerDeployment) {
	t.Helper()

	fs := testsupport.NewFakeStore()
	queue := testsupport.NewFakeEnqueuer()

	workerID := uuid.New()
	cloudID := uuid.New()
	finishedAt := time.Now().Add(-time.Minute)

	worker := &model.Worker{
		BaseModel:          model.BaseModel{ID: workerID},
		Name:               "demo-worker",
		Status:             model.WorkerStatusIdle,
		CloudAccountID:     &cloudID,
		R2ConfirmedBuckets: []string{},
	}
	parked := &model.WorkerDeployment{
		BaseModel:  model.BaseModel{ID: uuid.New()},
		WorkerID:   workerID,
		Status:     model.WorkerDeployNeedsConfirmation,
		R2Pending:  []model.WorkerR2Bucket{{Name: "my-cache", Empty: true}},
		FinishedAt: &finishedAt,
	}

	require.NoError(t, fs.WorkersStore.Create(context.Background(), worker))
	require.NoError(t, fs.WorkerDeploymentsStore.Create(context.Background(), parked))

	fs.WorkerDeploymentsStore.GetLatestByWorkerFn = func(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error) {
		if id != workerID {
			return nil, sql.ErrNoRows
		}
		rec, err := fs.WorkerDeploymentsStore.GetByID(ctx, parked.ID)
		if err != nil {
			return nil, err
		}
		copy := *rec
		return &copy, nil
	}

	return fs, queue, workerID, worker, parked
}

func TestConfirmR2_Success(t *testing.T) {
	fs, queue, workerID, worker, parked := seedWorkerConfirmFixture(t)
	s := newWorkerDeployService(fs, queue)

	newDep, err := s.ConfirmR2(context.Background(), workerID)
	require.NoError(t, err)
	require.NotNil(t, newDep)
	assert.Equal(t, model.WorkerDeployQueued, newDep.Status)
	assert.Equal(t, 1, queue.Calls)

	updatedWorker, err := fs.WorkersStore.GetByID(context.Background(), workerID)
	require.NoError(t, err)
	assert.Equal(t, []string{"my-cache"}, updatedWorker.R2ConfirmedBuckets)

	updatedParked, err := fs.WorkerDeploymentsStore.GetByID(context.Background(), parked.ID)
	require.NoError(t, err)
	assert.Equal(t, model.WorkerDeployCancelled, updatedParked.Status)
	assert.NotNil(t, updatedParked.FinishedAt)
	assert.Contains(t, updatedParked.DeployLog, "R2 bucket(s) confirmed; re-deploying.")

	updatedWorkerStatus, err := fs.WorkersStore.GetByID(context.Background(), worker.ID)
	require.NoError(t, err)
	assert.Equal(t, model.WorkerStatusDeploying, updatedWorkerStatus.Status)
}

func TestConfirmR2_TriggerFails_RevertsNeedsConfirmation(t *testing.T) {
	fs, queue, workerID, _, parked := seedWorkerConfirmFixture(t)
	queue.Err = errors.New("queue unavailable")
	s := newWorkerDeployService(fs, queue)

	_, err := s.ConfirmR2(context.Background(), workerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "queue unavailable")

	updatedParked, err := fs.WorkerDeploymentsStore.GetByID(context.Background(), parked.ID)
	require.NoError(t, err)
	assert.Equal(t, model.WorkerDeployNeedsConfirmation, updatedParked.Status)
	assert.Contains(t, updatedParked.DeployLog, "R2 confirmation failed; please try again.")
}

func TestConfirmR2_NoDeployment(t *testing.T) {
	fs := testsupport.NewFakeStore()
	queue := testsupport.NewFakeEnqueuer()
	s := newWorkerDeployService(fs, queue)

	workerID := uuid.New()
	cloudID := uuid.New()
	require.NoError(t, fs.WorkersStore.Create(context.Background(), &model.Worker{
		BaseModel:      model.BaseModel{ID: workerID},
		CloudAccountID: &cloudID,
		Status:         model.WorkerStatusIdle,
	}))

	fs.WorkerDeploymentsStore.GetLatestByWorkerFn = func(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error) {
		return nil, sql.ErrNoRows
	}

	_, err := s.ConfirmR2(context.Background(), workerID)
	require.Error(t, err)
	pd, ok := err.(*apierr.ProblemDetail)
	require.True(t, ok)
	assert.Equal(t, apierr.ErrNotFound.Status, pd.Status)
}

func TestConfirmR2_NotAwaitingConfirmation(t *testing.T) {
	fs, queue, workerID, _, _ := seedWorkerConfirmFixture(t)
	s := newWorkerDeployService(fs, queue)

	fs.WorkerDeploymentsStore.GetLatestByWorkerFn = func(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error) {
		return &model.WorkerDeployment{
			BaseModel: model.BaseModel{ID: uuid.New()},
			WorkerID:  workerID,
			Status:    model.WorkerDeployQueued,
		}, nil
	}

	_, err := s.ConfirmR2(context.Background(), workerID)
	require.Error(t, err)
	pd, ok := err.(*apierr.ProblemDetail)
	require.True(t, ok)
	assert.Equal(t, apierr.ErrConflict.Status, pd.Status)
}

func TestConfirmR2_DoubleConfirm_SecondFails(t *testing.T) {
	fs, queue, workerID, _, parked := seedWorkerConfirmFixture(t)
	s := newWorkerDeployService(fs, queue)

	_, err := s.ConfirmR2(context.Background(), workerID)
	require.NoError(t, err)

	fs.WorkerDeploymentsStore.GetLatestByWorkerFn = func(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error) {
		rec, err := fs.WorkerDeploymentsStore.GetByID(ctx, parked.ID)
		if err != nil {
			return nil, err
		}
		copy := *rec
		return &copy, nil
	}

	_, err = s.ConfirmR2(context.Background(), workerID)
	require.Error(t, err)
	pd, ok := err.(*apierr.ProblemDetail)
	require.True(t, ok)
	assert.Equal(t, apierr.ErrConflict.Status, pd.Status)
}
