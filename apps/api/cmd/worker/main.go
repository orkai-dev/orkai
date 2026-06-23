package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/uptrace/bun/migrate"

	"github.com/orkai-dev/orkai/apps/api/internal/config"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/leader"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator/k3s"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store/pg"
	"github.com/orkai-dev/orkai/apps/api/migrations"
)

const (
	visibilityTimeoutSec = 2400 // 40 min — longer than 30 min job cap
	maxReadAttempts      = 5
	pollInterval         = 2 * time.Second
	// maxConcurrentJobs bounds how many jobs run in parallel so a long-running
	// deploy does not block shorter jobs (e.g. a scheduled backup) behind it.
	maxConcurrentJobs = 4
	// drainTimeout is how long shutdown waits for in-flight jobs to finish on
	// their own before their context is cancelled. Anything still running past
	// this gets cancelled; its PGMQ visibility timeout then redelivers it to
	// another (or the next) worker, where job idempotency makes the retry safe.
	drainTimeout = 25 * time.Second
	// forceShutdownGrace bounds how long shutdown waits for jobs to unwind *after*
	// their context is cancelled. Without this cap a goroutine stuck in a
	// non-cancellable syscall would hang the worker forever; instead we log and
	// exit, relying on PGMQ redelivery.
	forceShutdownGrace = 5 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting orkai worker")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	secrets := secret.NewFromSetupSecret(cfg.Auth.SetupSecret)
	store, err := pg.New(cfg.Database.URL, secrets, pg.PoolConfig{
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait for migrations before touching the schema. The server also runs
	// migrations on boot; the shared advisory lock means whichever process
	// starts second simply blocks here and then runs a no-op migrate. Without
	// this, the worker can win the startup race in `make dev` and crash
	// querying tables (e.g. deploy_targets) that don't exist yet.
	if err := runMigrations(ctx, store, logger); err != nil {
		logger.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	queue := jobs.NewQueue(store.DB())
	if err := queue.EnsureSchema(ctx); err != nil {
		logger.Error("failed to ensure job queue schema", slog.Any("error", err))
		os.Exit(1)
	}

	targets, err := k3s.BootstrapTargetRegistry(ctx, store, cfg.K8s, logger)
	if err != nil {
		logger.Error("failed to bootstrap deploy targets", slog.Any("error", err))
		os.Exit(1)
	}

	metricsStore := pg.NewMetricsStore(store.DB())
	services := service.NewContainer(store, metricsStore, targets, nil, logger, cfg.Database.URL, cfg.Auth.SetupSecret, queue)

	// Leader election: only one worker replica runs singleton schedulers.
	workerElector := leader.NewElector(store.DB(), leader.WorkerLeaderKey, logger)
	workerElector.TryElect(ctx)
	go workerElector.Run(ctx)

	services.SystemBackup.StartScheduler(ctx, workerElector.IsLeader)
	services.Database.StartScheduler(ctx, workerElector.IsLeader)
	services.Deploy.StartStaleRecovery(ctx, workerElector.IsLeader)
	services.PageDeploy.StartStaleRecovery(ctx, workerElector.IsLeader)
	services.WorkerDeploy.StartStaleRecovery(ctx, workerElector.IsLeader)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("worker shutting down...")
		cancel()
	}()

	runConsumer(ctx, logger, queue, services)
	targets.Shutdown()
	logger.Info("worker exited gracefully")
}

// runMigrations applies pending migrations, guarded by a Postgres advisory
// lock shared with the API server so only one process migrates at a time.
func runMigrations(ctx context.Context, store *pg.Store, logger *slog.Logger) error {
	logger.Info("running database migrations...")

	if _, err := store.DB().ExecContext(ctx, "SELECT pg_advisory_lock(1)"); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		store.DB().ExecContext(ctx, "SELECT pg_advisory_unlock(1)") //nolint:errcheck
	}()

	migrator := migrate.NewMigrator(store.DB(), migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}
	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if group.IsZero() {
		logger.Info("no new migrations to run")
	} else {
		logger.Info("migrations applied", slog.String("group", group.String()))
	}
	return nil
}

func runConsumer(ctx context.Context, logger *slog.Logger, queue *jobs.Queue, services *service.Container) {
	// jobCtx drives in-flight job execution. It is deliberately NOT the
	// signal-cancelled ctx: on SIGTERM we stop reading new messages (ctx is
	// done) but let running jobs keep going so a mid-flight build is not aborted
	// the instant the worker receives the signal.
	jobCtx, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()

	// sem bounds concurrency; wg lets us drain in-flight jobs on shutdown.
	sem := make(chan struct{}, maxConcurrentJobs)
	var wg sync.WaitGroup

	// On return (shutdown), give in-flight jobs a bounded grace period to finish.
	// If they exceed it, cancel their context and wait for them to unwind; PGMQ
	// redelivery covers anything that gets cut off.
	defer func() {
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			logger.Info("all in-flight jobs drained")
			return
		case <-time.After(drainTimeout):
			logger.Warn("drain timeout exceeded, cancelling in-flight jobs", slog.Duration("timeout", drainTimeout))
			cancelJobs()
		}
		// Bounded wait after cancellation: don't hang forever on a goroutine
		// blocked in a non-cancellable operation — exit and let PGMQ redeliver.
		select {
		case <-done:
			logger.Info("in-flight jobs cancelled and drained")
		case <-time.After(forceShutdownGrace):
			logger.Warn("jobs did not exit after cancellation, forcing shutdown", slog.Duration("grace", forceShutdownGrace))
		}
	}()

	for {
		// Acquire a concurrency slot before reading so we never pull a message we
		// can't start working on (which would just burn its visibility timeout).
		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
		}

		msgs, err := queue.Read(ctx, visibilityTimeoutSec, 1)
		if err != nil {
			<-sem
			if ctx.Err() != nil {
				return
			}
			logger.Error("failed to read job queue", slog.Any("error", err))
			sleepOrDone(ctx, pollInterval)
			continue
		}

		if len(msgs) == 0 {
			<-sem
			sleepOrDone(ctx, pollInterval)
			continue
		}

		msg := msgs[0]
		wg.Add(1)
		go func(msg jobs.Message) {
			defer wg.Done()
			defer func() { <-sem }()
			processMessage(jobCtx, logger, queue, services, msg)
		}(msg)
	}
}

// processMessage dispatches a single job and removes/archives it from the queue.
func processMessage(ctx context.Context, logger *slog.Logger, queue *jobs.Queue, services *service.Container, msg jobs.Message) {
	if err := dispatchJob(ctx, logger, services, msg.Job); err != nil {
		logger.Error("job failed",
			slog.String("type", string(msg.Job.Type)),
			slog.Int64("msg_id", msg.MsgID),
			slog.Int("read_ct", msg.ReadCt),
			slog.Any("error", err),
		)
		if msg.ReadCt >= maxReadAttempts {
			if archErr := queue.Archive(context.Background(), msg.MsgID); archErr != nil {
				logger.Error("failed to archive poison job", slog.Any("error", archErr))
			}
		}
		return
	}

	if err := queue.Delete(context.Background(), msg.MsgID); err != nil {
		logger.Error("failed to delete processed job", slog.Int64("msg_id", msg.MsgID), slog.Any("error", err))
	}
}

func dispatchJob(ctx context.Context, logger *slog.Logger, services *service.Container, job jobs.Job) error {
	switch job.Type {
	case jobs.JobDeploy:
		if job.DeployID == nil {
			return fmt.Errorf("deploy job missing deploy_id")
		}
		logger.Info("processing deploy job", slog.String("deploy_id", job.DeployID.String()))
		return services.Deploy.RunDeployJob(ctx, *job.DeployID, job.ForceBuild)
	case jobs.JobSystemBackup:
		if job.BackupID == nil {
			return fmt.Errorf("backup job missing backup_id")
		}
		logger.Info("processing system backup job", slog.String("backup_id", job.BackupID.String()))
		return services.SystemBackup.RunBackupJob(ctx, *job.BackupID)
	case jobs.JobDatabaseBackup:
		if job.DatabaseBackupID == nil {
			return fmt.Errorf("database backup job missing database_backup_id")
		}
		logger.Info("processing database backup job", slog.String("database_backup_id", job.DatabaseBackupID.String()))
		return services.Database.RunDatabaseBackupJob(ctx, *job.DatabaseBackupID)
	case jobs.JobPageDeploy:
		if job.PageDeploymentID == nil {
			return fmt.Errorf("page deploy job missing page_deployment_id")
		}
		logger.Info("processing page deploy job", slog.String("page_deployment_id", job.PageDeploymentID.String()))
		return services.PageDeploy.RunJob(ctx, *job.PageDeploymentID)
	case jobs.JobWorkerDeploy:
		if job.WorkerDeploymentID == nil {
			return fmt.Errorf("worker deploy job missing worker_deployment_id")
		}
		logger.Info("processing worker deploy job", slog.String("worker_deployment_id", job.WorkerDeploymentID.String()))
		return services.WorkerDeploy.RunJob(ctx, *job.WorkerDeploymentID)
	default:
		return fmt.Errorf("unknown job type: %q", job.Type)
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
