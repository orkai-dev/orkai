package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type DatabaseService struct {
	store     store.Store
	targets   *orchestrator.TargetRegistry
	logger    *slog.Logger
	queue     Enqueuer
	providers *providers.Registry
	notifSvc  *NotificationService
}

func NewDatabaseService(s store.Store, targets *orchestrator.TargetRegistry, logger *slog.Logger, queue Enqueuer, prov *providers.Registry, notifSvc *NotificationService) *DatabaseService {
	return &DatabaseService{store: s, targets: targets, logger: logger, queue: queue, providers: prov, notifSvc: notifSvc}
}

type CreateDatabaseInput struct {
	ProjectID    uuid.UUID      `json:"project_id" binding:"required"`
	Name         string         `json:"name" binding:"required,min=1,max=63"`
	DatabaseName string         `json:"database_name"`
	Engine       model.DBEngine `json:"engine" binding:"required,oneof=postgres mysql mariadb redis valkey mongo"`
	Version      string         `json:"version" binding:"required"`
	StorageSize  string         `json:"storage_size"`
	CPULimit     string         `json:"cpu_limit"`
	MemLimit     string         `json:"mem_limit"`
}

// safeNameRe validates database/service names: alphanumeric, hyphens, underscores only.
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func (s *DatabaseService) Create(ctx context.Context, input CreateDatabaseInput) (*model.ManagedDatabase, error) {
	// Validate version
	if !model.IsValidVersion(input.Engine, input.Version) {
		return nil, fmt.Errorf("unsupported version %q for engine %q", input.Version, input.Engine)
	}

	// Validate name characters (prevents shell injection via $ORKAI_DB)
	if !safeNameRe.MatchString(input.Name) {
		return nil, fmt.Errorf("database name must start with alphanumeric and contain only letters, numbers, hyphens, and underscores")
	}

	dbName := input.DatabaseName
	if dbName == "" {
		dbName = input.Name // default database name = service name
	}
	if !safeNameRe.MatchString(dbName) {
		return nil, fmt.Errorf("database name %q contains invalid characters", dbName)
	}

	// Inherit namespace from project
	project, err := s.store.Projects().GetByID(ctx, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if project.Namespace == "" {
		return nil, fmt.Errorf("project has no namespace configured")
	}

	db := &model.ManagedDatabase{
		ProjectID:    input.ProjectID,
		Name:         input.Name,
		DatabaseName: dbName,
		Engine:       input.Engine,
		Version:      input.Version,
		StorageSize:  input.StorageSize,
		CPULimit:     input.CPULimit,
		MemLimit:     input.MemLimit,
		Namespace:    project.Namespace,
		Status:       model.AppStatusIdle,
	}

	if db.StorageSize == "" {
		db.StorageSize = "1Gi"
	}
	if db.CPULimit == "" {
		db.CPULimit = "500m"
	}
	if db.MemLimit == "" {
		db.MemLimit = "512Mi"
	}

	// Check for K8s name conflicts in the same project (apps + databases share
	// namespace). Both lookups are single indexed (project_id, k8s_name) seeks.
	k8sName := sanitizeK8sName(db.Name)
	db.K8sName = k8sName
	if dup, err := s.store.ManagedDatabases().ExistsByK8sName(ctx, input.ProjectID, k8sName); err != nil {
		return nil, fmt.Errorf("cannot verify name uniqueness: %w", err)
	} else if dup {
		return nil, fmt.Errorf("a database with K8s name %q already exists", k8sName)
	}
	if dup, err := s.store.Applications().ExistsByK8sName(ctx, input.ProjectID, k8sName); err != nil {
		return nil, fmt.Errorf("cannot verify name uniqueness: %w", err)
	} else if dup {
		return nil, fmt.Errorf("an application with K8s name %q already exists — app and database names must not collide", k8sName)
	}

	if err := s.store.ManagedDatabases().Create(ctx, db); err != nil {
		return nil, err
	}

	// Deploy to K3s
	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		_ = s.store.ManagedDatabases().Delete(ctx, db.ID)
		return nil, err
	}
	if err := dbMgr.DeployDatabase(ctx, db); err != nil {
		s.logger.Error("failed to deploy database", slog.Any("error", err))
		_ = s.store.ManagedDatabases().Delete(ctx, db.ID)
		return nil, err
	}

	// Persist K8s metadata set by orchestrator
	if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
		s.logger.Error("failed to update database with k8s metadata", slog.Any("error", err))
	}

	s.logger.Info("managed database created",
		slog.String("name", db.Name),
		slog.String("engine", string(db.Engine)),
	)
	return db, nil
}

func (s *DatabaseService) GetByID(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
	return s.store.ManagedDatabases().GetByID(ctx, id)
}

func (s *DatabaseService) List(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.ManagedDatabase, int, error) {
	dbs, total, err := s.store.ManagedDatabases().ListByProject(ctx, projectID, params)
	if err == nil {
		s.syncLiveStatuses(ctx, dbs)
	}
	return dbs, total, err
}

func (s *DatabaseService) ListAll(ctx context.Context, params store.ListParams, filter store.DatabaseListFilter) ([]model.ManagedDatabase, int, error) {
	dbs, total, err := s.store.ManagedDatabases().ListAll(ctx, params, filter)
	if err == nil {
		s.syncLiveStatuses(ctx, dbs)
	}
	return dbs, total, err
}

func (s *DatabaseService) syncLiveStatuses(ctx context.Context, dbs []model.ManagedDatabase) {
	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return
	}
	for i := range dbs {
		if dbs[i].Status == model.AppStatusIdle {
			continue
		}
		status, err := dbMgr.GetDatabaseStatus(ctx, &dbs[i])
		if err != nil {
			continue
		}
		var live model.AppStatus
		switch status.Phase {
		case "running":
			live = model.AppStatusRunning
		case "stopped":
			live = model.AppStatusStopped
		case "not deployed":
			if dbs[i].Status != model.AppStatusIdle {
				live = model.AppStatusError
			}
		case "pending":
			live = model.AppStatusDeploying
		}
		if live != "" && live != dbs[i].Status {
			dbs[i].Status = live
			go func(db model.ManagedDatabase, s2 model.AppStatus) {
				db.Status = s2
				_ = s.store.ManagedDatabases().Update(context.Background(), &db)
			}(dbs[i], live)
		}
	}
}

func (s *DatabaseService) Delete(ctx context.Context, id uuid.UUID) error {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return err
	}

	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return err
	}

	if err := dbMgr.DeleteDatabase(ctx, db); err != nil {
		s.logger.Error("failed to delete database from orchestrator", slog.Any("error", err))
		return fmt.Errorf("failed to delete database resources: %w — delete manually from K8s before retrying", err)
	}

	// Also clean up external access service if enabled
	if db.ExternalEnabled {
		if err := dbMgr.DisableExternalAccess(ctx, db); err != nil {
			s.logger.Warn("failed to cleanup external access", slog.Any("error", err))
		}
	}

	if err := s.store.ManagedDatabases().Delete(ctx, id); err != nil {
		return err
	}

	notifyProjectResourceDeleted(s.notifSvc, s.store, s.logger, db.ProjectID, model.EventDatabaseDeleted,
		db.Name, fmt.Sprintf("Database %q was deleted", db.Name))
	return nil
}

func (s *DatabaseService) GetCredentials(ctx context.Context, id uuid.UUID) (*orchestrator.DatabaseCredentials, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return nil, err
	}
	return dbMgr.GetDatabaseCredentials(ctx, db)
}

func (s *DatabaseService) GetStatus(ctx context.Context, id uuid.UUID) (*orchestrator.AppStatus, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return nil, err
	}
	status, err := dbMgr.GetDatabaseStatus(ctx, db)
	if err != nil {
		return nil, err
	}

	// Reconcile: update DB record if K8s state differs
	newStatus := db.Status
	switch status.Phase {
	case "running":
		newStatus = model.AppStatusRunning
	case "not deployed":
		if db.Status == model.AppStatusIdle {
			// Fresh database that was never deployed — keep idle
			newStatus = model.AppStatusIdle
		} else {
			// Previously deployed database with missing resources — flag as error
			newStatus = model.AppStatusError
		}
	case "pending":
		newStatus = "pending"
	case "stopped":
		newStatus = model.AppStatusStopped
	}
	if newStatus != db.Status {
		db.Status = newStatus
		if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
			s.logger.Error("failed to reconcile database status", slog.Any("error", err))
		}
	}
	return status, nil
}

func (s *DatabaseService) GetPods(ctx context.Context, id uuid.UUID) ([]orchestrator.PodInfo, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return nil, err
	}
	return dbMgr.GetDatabasePods(ctx, db)
}

func (s *DatabaseService) UpdateExternalAccess(ctx context.Context, id uuid.UUID, enabled bool, port int32) (*model.ManagedDatabase, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return nil, err
	}

	if enabled {
		if port > 0 && (port < 30000 || port > 32767) {
			return nil, fmt.Errorf("NodePort must be between 30000–32767, got %d", port)
		}
		if port > 0 {
			// Check port conflict with other databases
			conflict, err := s.store.ManagedDatabases().FindByExternalPort(ctx, port)
			if err == nil && conflict != nil && conflict.ID != id {
				return nil, fmt.Errorf("port %d is already used by database %q", port, conflict.Name)
			}
			db.ExternalPort = port
		}
		nodePort, err := dbMgr.EnableExternalAccess(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("enable external access: %w", err)
		}
		db.ExternalEnabled = true
		db.ExternalPort = nodePort
	} else {
		if err := dbMgr.DisableExternalAccess(ctx, db); err != nil {
			return nil, fmt.Errorf("disable external access: %w", err)
		}
		db.ExternalEnabled = false
		db.ExternalPort = 0
	}

	if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
		return nil, fmt.Errorf("update database: %w", err)
	}

	s.logger.Info("database external access updated",
		slog.String("database", db.Name),
		slog.Bool("enabled", enabled),
		slog.Int("port", int(db.ExternalPort)),
	)
	return db, nil
}

// UpdateBackupInput holds the configuration for database backup settings.
type UpdateBackupInput struct {
	Enabled  bool       `json:"enabled"`
	Schedule string     `json:"schedule"`
	S3ID     *uuid.UUID `json:"s3_id"`
}

func (s *DatabaseService) UpdateBackupConfig(ctx context.Context, id uuid.UUID, input UpdateBackupInput) (*model.ManagedDatabase, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Validate cron schedule if backup is being enabled
	if input.Enabled && input.Schedule != "" {
		fields := strings.Fields(input.Schedule)
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid cron schedule %q: expected 5 fields (minute hour dom month dow)", input.Schedule)
		}
	}

	// Validate S3 resource exists, is correct type, and belongs to same org
	if input.S3ID != nil {
		resource, err := s.store.SharedResources().GetByID(ctx, *input.S3ID)
		if err != nil {
			return nil, fmt.Errorf("S3 resource not found: %w", err)
		}
		if resource.Type != model.ResourceObjectStorage {
			return nil, fmt.Errorf("resource %s is not an object storage resource", resource.Name)
		}
		project, projErr := s.store.Projects().GetByID(ctx, db.ProjectID)
		if projErr != nil {
			return nil, fmt.Errorf("project not found: %w", projErr)
		}
		if resource.OrgID != project.OrgID {
			return nil, fmt.Errorf("S3 resource does not belong to this organization")
		}
	}

	db.BackupEnabled = input.Enabled
	db.BackupSchedule = input.Schedule
	db.BackupS3ID = input.S3ID

	if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
		return nil, fmt.Errorf("update database: %w", err)
	}

	s.logger.Info("database backup config updated",
		slog.String("database", db.Name),
		slog.Bool("enabled", input.Enabled),
		slog.String("schedule", input.Schedule),
	)
	return db, nil
}

// UsedExternalPorts returns all ports currently in use for external access.
func (s *DatabaseService) UsedExternalPorts(ctx context.Context) ([]model.ExternalPortInfo, error) {
	return s.store.ManagedDatabases().ListExternalPorts(ctx)
}

// StartScheduler runs a ticker loop that fires scheduled backups for every
// managed database whose cron schedule matches the current minute. It mirrors
// SystemBackupService.StartScheduler and reuses the same cron-matching helpers.
// When isLeader is non-nil, only the elected leader triggers backups.
func (s *DatabaseService) StartScheduler(ctx context.Context, isLeader func() bool) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if shouldRunSingleton(isLeader) {
					// runDueBackups only creates records + enqueues jobs now, so it
					// returns quickly; the readiness wait and upload happen in the
					// worker's RunDatabaseBackupJob. Pass the worker ctx so a SIGTERM
					// stops dispatching new scheduled backups during shutdown.
					s.runDueBackups(ctx)
				}
			}
		}
	}()
}

// runDueBackups triggers a backup for each enabled database whose schedule is
// due now. Failures are logged per-database so one bad schedule does not stop
// the others.
func (s *DatabaseService) runDueBackups(ctx context.Context) {
	dbs, err := s.store.ManagedDatabases().ListBackupEnabled(ctx)
	if err != nil {
		s.logger.Error("failed to list backup-enabled databases", slog.Any("error", err))
		return
	}
	for i := range dbs {
		db := dbs[i]
		if db.BackupSchedule == "" || !shouldRunNow(db.BackupSchedule) {
			continue
		}
		s.logger.Info("triggering scheduled database backup",
			slog.String("database", db.Name),
			slog.String("schedule", db.BackupSchedule),
		)
		if _, err := s.TriggerBackup(ctx, db.ID); err != nil {
			s.logger.Error("scheduled database backup failed",
				slog.String("database", db.Name),
				slog.Any("error", err),
			)
		}
	}
}

// dbBackupExt returns the file extension for a database engine backup.
func dbBackupExt(engine model.DBEngine) string {
	switch engine {
	case model.DBPostgres, model.DBMySQL, model.DBMariaDB:
		return "sql"
	case model.DBMongo:
		return "archive"
	case model.DBRedis, model.DBValkey:
		return "rdb"
	default:
		return "dump"
	}
}

// dbSafeName returns the K8s name or database name, preferring K8sName.
func dbSafeName(db *model.ManagedDatabase) string {
	if db.K8sName != "" {
		return db.K8sName
	}
	return db.Name
}

type backupStorage struct {
	Provider string
	Config   json.RawMessage
}

func (s *DatabaseService) loadBackupStorage(ctx context.Context, s3ID *uuid.UUID) (*backupStorage, error) {
	if s3ID == nil {
		return nil, nil
	}

	resource, err := s.store.SharedResources().GetByID(ctx, *s3ID)
	if err != nil {
		return nil, fmt.Errorf("load S3 resource: %w", err)
	}

	// Resolve cloud-account-backed credentials at use time so IAM key rotation
	// on the parent account propagates without re-saving this resource.
	cfg, err := resolveObjectStorageConfig(ctx, s.store, resource.OrgID, resource.Provider, resource.Config)
	if err != nil {
		return nil, fmt.Errorf("resolve S3 credentials: %w", err)
	}

	return &backupStorage{Provider: resource.Provider, Config: cfg}, nil
}

// TriggerBackup creates a backup record and enqueues a durable job. The actual
// readiness wait, S3 upload, and K8s job launch happen in RunDatabaseBackupJob
// on the worker so the HTTP request returns immediately instead of blocking
// through ~30s of readiness polling.
func (s *DatabaseService) TriggerBackup(ctx context.Context, databaseID uuid.UUID) (*model.DatabaseBackup, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, databaseID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	backup := &model.DatabaseBackup{
		DatabaseID: databaseID,
		Status:     "pending",
		StartedAt:  &now,
	}

	if err := s.store.DatabaseBackups().Create(ctx, backup); err != nil {
		return nil, fmt.Errorf("create backup record: %w", err)
	}

	if err := s.queue.Enqueue(ctx, jobs.NewDatabaseBackupJob(backup.ID)); err != nil {
		// The backup row was already created. There is no stale-backup recovery
		// for managed-DB backups, so a failed enqueue would leave it stuck at
		// "pending" forever. Mirror the deploy/system-backup pattern and mark it
		// failed before returning.
		finished := time.Now()
		backup.Status = "failed"
		backup.FinishedAt = &finished
		if updErr := s.store.DatabaseBackups().Update(ctx, backup); updErr != nil {
			s.logger.Error("failed to mark database backup failed after enqueue error", slog.Any("error", updErr))
		}
		return nil, fmt.Errorf("enqueue database backup job: %w", err)
	}

	s.logger.Info("database backup queued",
		slog.String("database", db.Name),
		slog.String("backup_id", backup.ID.String()),
	)
	return backup, nil
}

// RunDatabaseBackupJob executes a managed-database backup job from the worker
// queue: it waits for the database to become ready, then launches the K8s backup
// job. The K8s job itself runs asynchronously; ListBackups reconciles its final
// status. Returns an error only for transient failures that warrant a retry;
// terminal failures are recorded on the backup row and return nil so the message
// is not redelivered.
func (s *DatabaseService) RunDatabaseBackupJob(ctx context.Context, backupID uuid.UUID) error {
	backup, err := s.store.DatabaseBackups().GetByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("load backup: %w", err)
	}

	if backup.Status == "completed" || backup.Status == "failed" {
		s.logger.Info("skipping database backup job — already terminal",
			slog.String("backup_id", backupID.String()),
			slog.String("status", backup.Status),
		)
		return nil
	}

	db, err := s.store.ManagedDatabases().GetByID(ctx, backup.DatabaseID)
	if err != nil {
		return s.failDatabaseBackup(ctx, backup, fmt.Sprintf("database not found: %v", err))
	}

	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return s.failDatabaseBackup(ctx, backup, err.Error())
	}

	// A record already in "running" means this is a redelivery — the worker
	// crashed between marking it running and deleting the queue message. The K8s
	// backup job was already launched (or is about to be) under the original
	// FilePath. Reconcile from the live job instead of relaunching with a fresh
	// S3 key, which would desync the restore path. If the job already reached a
	// terminal state, record it; otherwise leave it "running" for ListBackups to
	// reconcile, but still fall through to an idempotent (re)launch so a crash
	// that happened *before* the job was created does not strand the record.
	if backup.Status == "running" {
		switch dbMgr.GetBackupJobStatus(ctx, backup.ID) {
		case "completed":
			now := time.Now()
			backup.Status = "completed"
			backup.FinishedAt = &now
			if err := s.store.DatabaseBackups().Update(ctx, backup); err != nil {
				return fmt.Errorf("reconcile backup completed: %w", err)
			}
			s.logger.Info("database backup reconciled completed on redelivery", slog.String("backup_id", backup.ID.String()))
			return nil
		case "failed":
			return s.failDatabaseBackup(ctx, backup, "backup K8s job reported failure")
		}
	}

	// Verify the database is actually deployed and running in K8s before backup.
	// If pending, wait up to 30s for it to become ready (covers freshly-created databases).
	var status *orchestrator.AppStatus
	for attempt := 0; attempt < 6; attempt++ {
		status, err = dbMgr.GetDatabaseStatus(ctx, db)
		if err != nil {
			return s.failDatabaseBackup(ctx, backup, fmt.Sprintf("cannot check database status: %v", err))
		}
		if status.Phase == "running" {
			break
		}
		if status.Phase == "not deployed" {
			return s.failDatabaseBackup(ctx, backup, fmt.Sprintf("database %q is not deployed — deploy it first before backing up", db.Name))
		}
		// pending/other — wait 5s and retry
		if attempt < 5 {
			s.logger.Info("backup waiting for database to become ready",
				slog.String("db", db.Name), slog.String("phase", status.Phase), slog.Int("attempt", attempt+1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	if status.Phase != "running" {
		return s.failDatabaseBackup(ctx, backup, fmt.Sprintf("database %q is not ready after 30s (status: %s)", db.Name, status.Phase))
	}

	// Load object-storage config if configured
	storage, err := s.loadBackupStorage(ctx, db.BackupS3ID)
	if err != nil {
		return s.failDatabaseBackup(ctx, backup, fmt.Sprintf("load S3 config: %v", err))
	}

	// Reuse the S3 key on a redelivery (FilePath already set) so the restore path
	// stays in sync with the file the original K8s job is writing. Otherwise
	// generate a stable key: {engine}-{name}/{timestamp}-{id8}.{ext}.
	s3Key := backup.FilePath
	if s3Key == "" {
		backupExt := dbBackupExt(db.Engine)
		ts := time.Now().UTC().Format("20060102-150405")
		s3Key = fmt.Sprintf("orkai/db-backups/%s-%s/%s-%s.%s",
			db.Engine, dbSafeName(db), ts, backup.ID.String()[:8], backupExt)
		backup.FilePath = s3Key
	}
	backup.Status = "running"
	if err := s.store.DatabaseBackups().Update(ctx, backup); err != nil {
		return fmt.Errorf("mark backup running: %w", err)
	}

	var transfer *orchestrator.ObjectTransfer
	if storage != nil {
		backupFile := fmt.Sprintf("/backup/%s.%s", backup.ID.String(), dbBackupExt(db.Engine))
		t, err := s.providers.ObjectStorage(storage.Provider).UploadJob(storage.Config, backupFile, s3Key)
		if err != nil {
			return s.failDatabaseBackup(ctx, backup, fmt.Sprintf("build upload job: %v", err))
		}
		transfer = &t
	}

	if err := dbMgr.RunDatabaseBackup(ctx, db, backup.ID, transfer); err != nil {
		return s.failDatabaseBackup(ctx, backup, fmt.Sprintf("run backup: %v", err))
	}

	s.logger.Info("database backup launched",
		slog.String("database", db.Name),
		slog.String("backup_id", backup.ID.String()),
		slog.String("s3_key", s3Key),
	)
	return nil
}

// failDatabaseBackup marks a backup record failed and returns nil so the queue
// message is deleted rather than redelivered. The DatabaseBackup model has no
// error column, so the cause is logged.
func (s *DatabaseService) failDatabaseBackup(ctx context.Context, backup *model.DatabaseBackup, msg string) error {
	now := time.Now()
	backup.Status = "failed"
	backup.FinishedAt = &now
	if err := s.store.DatabaseBackups().Update(ctx, backup); err != nil {
		s.logger.Error("failed to mark database backup failed", slog.Any("error", err))
	}
	s.logger.Error("database backup failed",
		slog.String("backup_id", backup.ID.String()),
		slog.String("error", msg),
	)
	return nil
}

func (s *DatabaseService) ListBackups(ctx context.Context, databaseID uuid.UUID, params store.ListParams) ([]model.DatabaseBackup, int, error) {
	backups, total, err := s.store.DatabaseBackups().ListByDatabase(ctx, databaseID, params)
	if err != nil {
		return nil, 0, err
	}

	// Reconcile: check K8s Job status for any "running" backups
	dbMgr, err := targetDatabase(s.targets)
	if err == nil {
		for i := range backups {
			if backups[i].Status != "running" {
				continue
			}
			jobStatus := dbMgr.GetBackupJobStatus(ctx, backups[i].ID)
			if jobStatus == "" {
				continue
			}
			now := time.Now()
			switch jobStatus {
			case "completed":
				backups[i].Status = "completed"
				backups[i].FinishedAt = &now
			case "failed":
				backups[i].Status = "failed"
				backups[i].FinishedAt = &now
			}
			_ = s.store.DatabaseBackups().Update(ctx, &backups[i])
		}

		// Also reconcile restore job statuses
		for i := range backups {
			if backups[i].RestoreStatus != "running" {
				continue
			}
			jobStatus := dbMgr.GetRestoreJobStatus(ctx, backups[i].ID)
			if jobStatus == "" {
				continue
			}
			switch jobStatus {
			case "completed":
				backups[i].RestoreStatus = "completed"
			case "failed":
				backups[i].RestoreStatus = "failed"
			}
			_ = s.store.DatabaseBackups().Update(ctx, &backups[i])
		}
	}

	return backups, total, nil
}

func (s *DatabaseService) RestoreBackup(ctx context.Context, databaseID, backupID uuid.UUID) error {
	db, err := s.store.ManagedDatabases().GetByID(ctx, databaseID)
	if err != nil {
		return err
	}

	// Verify the database is running
	dbMgr, err := targetDatabase(s.targets)
	if err != nil {
		return err
	}
	status, err := dbMgr.GetDatabaseStatus(ctx, db)
	if err != nil {
		return fmt.Errorf("cannot check database status: %w", err)
	}
	if status.Phase != "running" {
		return fmt.Errorf("database %q must be running to restore (current: %s)", db.Name, status.Phase)
	}

	// Get the backup record
	backup, err := s.store.DatabaseBackups().GetByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}
	if backup.DatabaseID != databaseID {
		return fmt.Errorf("backup does not belong to this database")
	}
	if backup.Status != "completed" {
		return fmt.Errorf("can only restore from a completed backup (current: %s)", backup.Status)
	}
	if backup.RestoreStatus == "running" {
		return fmt.Errorf("a restore is already in progress for this backup")
	}

	// Load object-storage config
	storage, err := s.loadBackupStorage(ctx, db.BackupS3ID)
	if err != nil {
		return fmt.Errorf("load S3 config: %w", err)
	}
	if storage == nil {
		return fmt.Errorf("S3 storage is not configured for this database — configure backup settings first")
	}

	// Use saved S3 key from backup record; fall back to legacy path for old backups
	s3Key := backup.FilePath
	if s3Key == "" {
		ext := dbBackupExt(db.Engine)
		s3Key = fmt.Sprintf("orkai/%s/%s.%s", db.Name, backupID.String(), ext)
	}

	backupFile := fmt.Sprintf("/backup/%s.%s", backupID.String(), dbBackupExt(db.Engine))
	transfer, err := s.providers.ObjectStorage(storage.Provider).DownloadJob(storage.Config, s3Key, backupFile)
	if err != nil {
		return fmt.Errorf("build download job: %w", err)
	}

	// Launch restore job
	if err := dbMgr.RestoreDatabaseBackup(ctx, db, backupID, &transfer); err != nil {
		return fmt.Errorf("start restore job: %w", err)
	}

	// Update backup record with restore status
	backup.RestoreStatus = "running"
	if err := s.store.DatabaseBackups().Update(ctx, backup); err != nil {
		s.logger.Error("failed to update backup restore status", slog.Any("error", err))
	}

	s.logger.Info("database restore started",
		slog.String("database", db.Name),
		slog.String("backup_id", backupID.String()),
	)
	return nil
}
