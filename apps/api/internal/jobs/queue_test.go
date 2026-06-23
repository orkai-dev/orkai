//go:build integration

package jobs_test

import (
	"context"
	"database/sql"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
)

func dockerAvailable() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

func newTestQueue(t *testing.T) (*jobs.Queue, func()) {
	t.Helper()
	if !dockerAvailable() {
		t.Skip("docker not available; skipping integration test")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"ghcr.io/pgmq/pg18-pgmq:v1.11.1",
		tcpostgres.WithDatabase("orkai"),
		tcpostgres.WithUsername("orkai"),
		tcpostgres.WithPassword("orkai"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db := bun.NewDB(sqldb, pgdialect.New())
	queue := jobs.NewQueue(db)

	cleanup := func() {
		_ = db.Close()
		_ = container.Terminate(ctx)
	}
	return queue, cleanup
}

func TestQueueEnsureSchemaIdempotent(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, queue.EnsureSchema(ctx))
	require.NoError(t, queue.EnsureSchema(ctx))
}

func TestQueueEnqueueReadDelete(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, queue.EnsureSchema(ctx))

	deployID := uuid.New()
	require.NoError(t, queue.Enqueue(ctx, jobs.NewDeployJob(deployID, true)))

	msgs, err := queue.Read(ctx, 30, 1)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, jobs.JobDeploy, msgs[0].Job.Type)
	require.NotNil(t, msgs[0].Job.DeployID)
	assert.Equal(t, deployID, *msgs[0].Job.DeployID)
	assert.True(t, msgs[0].Job.ForceBuild)

	require.NoError(t, queue.Delete(ctx, msgs[0].MsgID))

	msgs, err = queue.Read(ctx, 30, 1)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestQueueVisibilityTimeout(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, queue.EnsureSchema(ctx))

	require.NoError(t, queue.Enqueue(ctx, jobs.NewSystemBackupJob(uuid.New())))

	msgs, err := queue.Read(ctx, 2, 1)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	invisible, err := queue.Read(ctx, 2, 1)
	require.NoError(t, err)
	assert.Empty(t, invisible)

	time.Sleep(3 * time.Second)

	visible, err := queue.Read(ctx, 2, 1)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	assert.Equal(t, msgs[0].MsgID, visible[0].MsgID)
}

func TestQueueArchive(t *testing.T) {
	queue, cleanup := newTestQueue(t)
	defer cleanup()
	ctx := context.Background()
	require.NoError(t, queue.EnsureSchema(ctx))

	require.NoError(t, queue.Enqueue(ctx, jobs.NewPageDeployJob(uuid.New())))

	msgs, err := queue.Read(ctx, 30, 1)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	require.NoError(t, queue.Archive(ctx, msgs[0].MsgID))

	msgs, err = queue.Read(ctx, 30, 1)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}
