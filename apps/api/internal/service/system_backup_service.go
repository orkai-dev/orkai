package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// SystemBackupService manages backups of the Orkai PostgreSQL database to S3.
type SystemBackupService struct {
	store     store.Store
	settings  *SettingService
	dbURL     string
	logger    *slog.Logger
	queue     Enqueuer
	providers *providers.Registry
}

// SystemBackupConfig holds the configuration for system backups.
type SystemBackupConfig struct {
	Enabled   bool   `json:"enabled"`
	S3ID      string `json:"s3_id"`
	Schedule  string `json:"schedule"`
	Path      string `json:"path"`
	Retention int    `json:"retention"`
}

// NewSystemBackupService creates a new SystemBackupService.
func NewSystemBackupService(s store.Store, settings *SettingService, dbURL string, logger *slog.Logger, queue Enqueuer, prov *providers.Registry) *SystemBackupService {
	return &SystemBackupService{
		store:     s,
		settings:  settings,
		dbURL:     dbURL,
		logger:    logger,
		queue:     queue,
		providers: prov,
	}
}

// GetConfig reads system backup configuration from settings.
func (s *SystemBackupService) GetConfig(ctx context.Context) (*SystemBackupConfig, error) {
	enabled, _ := s.store.Settings().Get(ctx, "system_backup_enabled")
	s3ID, _ := s.store.Settings().Get(ctx, "system_backup_s3_id")
	schedule, _ := s.store.Settings().Get(ctx, "system_backup_schedule")
	path, _ := s.store.Settings().Get(ctx, "system_backup_path")
	retentionStr, _ := s.store.Settings().Get(ctx, "system_backup_retention")

	retention := 30
	if retentionStr != "" {
		if v, err := strconv.Atoi(retentionStr); err == nil && v > 0 {
			retention = v
		}
	}

	return &SystemBackupConfig{
		Enabled:   enabled == "true",
		S3ID:      s3ID,
		Schedule:  schedule,
		Path:      path,
		Retention: retention,
	}, nil
}

// SaveConfig writes system backup configuration to settings.
func (s *SystemBackupService) SaveConfig(ctx context.Context, cfg *SystemBackupConfig) error {
	// When enabling, require S3 and schedule
	if cfg.Enabled {
		if cfg.S3ID == "" {
			return fmt.Errorf("S3 storage resource is required when backups are enabled")
		}
		if cfg.Schedule == "" {
			return fmt.Errorf("backup schedule is required when backups are enabled")
		}
		fields := strings.Fields(cfg.Schedule)
		if len(fields) != 5 {
			return fmt.Errorf("invalid cron schedule %q: expected 5 fields", cfg.Schedule)
		}
	}

	// Validate S3 resource exists and is correct type
	if cfg.S3ID != "" {
		s3UUID, parseErr := uuid.Parse(cfg.S3ID)
		if parseErr != nil {
			return fmt.Errorf("invalid S3 resource ID: %w", parseErr)
		}
		resource, resErr := s.store.SharedResources().GetByID(ctx, s3UUID)
		if resErr != nil {
			return fmt.Errorf("S3 resource not found: %w", resErr)
		}
		if resource.Type != model.ResourceObjectStorage {
			return fmt.Errorf("resource %q is not an object storage resource", resource.Name)
		}
	}

	pairs := map[string]string{
		"system_backup_s3_id":     cfg.S3ID,
		"system_backup_schedule":  cfg.Schedule,
		"system_backup_path":      cfg.Path,
		"system_backup_retention": strconv.Itoa(cfg.Retention),
	}
	if cfg.Enabled {
		pairs["system_backup_enabled"] = "true"
	} else {
		pairs["system_backup_enabled"] = "false"
	}
	for k, v := range pairs {
		if err := s.store.Settings().Set(ctx, k, v); err != nil {
			return err
		}
	}
	s.logger.Info("system backup config updated")
	return nil
}

// TriggerBackup starts a system database backup.
func (s *SystemBackupService) TriggerBackup(ctx context.Context) (*model.SystemBackup, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup config: %w", err)
	}
	if cfg.S3ID == "" {
		return nil, fmt.Errorf("system backup S3 resource not configured")
	}

	// Load S3 config from shared_resources
	s3ID, err := uuid.Parse(cfg.S3ID)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 resource ID: %w", err)
	}
	resource, err := s.store.SharedResources().GetByID(ctx, s3ID)
	if err != nil {
		return nil, fmt.Errorf("S3 resource not found: %w", err)
	}

	bucket, err := s.providers.ObjectStorage(resource.Provider).Bucket(resource.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid object storage config: %w", err)
	}

	now := time.Now()
	backup := &model.SystemBackup{
		Status:    "queued",
		StartedAt: &now,
		S3Bucket:  bucket,
	}
	if err := s.store.SystemBackups().Create(ctx, backup); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	if err := s.queue.Enqueue(ctx, jobs.NewSystemBackupJob(backup.ID)); err != nil {
		// The backup row was already created. There is no stale-backup recovery, so
		// a failed enqueue would leave it stuck at "queued" forever. Mirror the
		// deploy pattern and mark it failed before returning.
		finishedAt := time.Now()
		backup.Status = "failed"
		backup.FinishedAt = &finishedAt
		backup.Error = fmt.Sprintf("Failed to enqueue backup job: %v", err)
		if updErr := s.store.SystemBackups().Update(ctx, backup); updErr != nil {
			s.logger.Error("failed to mark backup failed after enqueue error", slog.Any("error", updErr))
		}
		return nil, fmt.Errorf("enqueue backup job: %w", err)
	}

	return backup, nil
}

// RunBackupJob executes a system backup job from the worker queue.
func (s *SystemBackupService) RunBackupJob(ctx context.Context, backupID uuid.UUID) error {
	backup, err := s.store.SystemBackups().GetByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("load backup: %w", err)
	}

	if backup.Status == "completed" || backup.Status == "failed" {
		s.logger.Info("skipping backup job — already terminal",
			slog.String("backup_id", backupID.String()),
			slog.String("status", backup.Status),
		)
		return nil
	}

	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("load backup config: %w", err)
	}
	if cfg.S3ID == "" {
		return fmt.Errorf("system backup S3 resource not configured")
	}

	s3ID, err := uuid.Parse(cfg.S3ID)
	if err != nil {
		return fmt.Errorf("invalid S3 resource ID: %w", err)
	}
	resource, err := s.store.SharedResources().GetByID(ctx, s3ID)
	if err != nil {
		return fmt.Errorf("S3 resource not found: %w", err)
	}
	// Resolve cloud-account-backed credentials at use time (in-memory only) so
	// IAM key rotation on the parent account propagates without re-saving.
	resolved, err := resolveObjectStorageConfig(ctx, s.store, resource.OrgID, resource.Provider, resource.Config)
	if err != nil {
		return fmt.Errorf("resolve S3 credentials: %w", err)
	}
	resource.Config = resolved

	backup.Status = "running"
	if err := s.store.SystemBackups().Update(ctx, backup); err != nil {
		return fmt.Errorf("mark backup running: %w", err)
	}

	s.runBackup(ctx, backup, cfg, resource)
	return nil
}

// ListBackups returns a paginated list of system backups.
func (s *SystemBackupService) ListBackups(ctx context.Context, params store.ListParams) ([]model.SystemBackup, int, error) {
	return s.store.SystemBackups().List(ctx, params)
}

// runBackup performs the actual pg_dump and object-storage upload. The caller's
// context is honoured so worker shutdown (SIGTERM) interrupts in-flight pg_dump.
func (s *SystemBackupService) runBackup(ctx context.Context, backup *model.SystemBackup, cfg *SystemBackupConfig, resource *model.SharedResource) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	timestamp := time.Now().UTC().Format("20060102-150405")
	fileName := fmt.Sprintf("orkai-%s.dump", timestamp)

	s3Path := cfg.Path
	if s3Path == "" {
		s3Path = "orkai-backups"
	}
	fullS3Path := fmt.Sprintf("%s/%s", s3Path, fileName)
	backup.S3Path = fullS3Path
	backup.FileName = fileName

	// Create temp file for dump
	tmpFile, err := os.CreateTemp("", "orkai-backup-*.dump")
	if err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("failed to create temp file: %v", err))
		return
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	// Parse database URL
	host, port, user, password, dbname, parseErr := parseDatabaseURL(s.dbURL)
	if parseErr != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("failed to parse database URL: %v", parseErr))
		return
	}

	// Run pg_dump
	cmd := exec.CommandContext(ctx, "pg_dump",
		"-h", host, "-p", port, "-U", user, "-d", dbname,
		"-F", "c",
		"-f", tmpPath,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	if output, err := cmd.CombinedOutput(); err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("pg_dump failed: %v — %s", err, string(output)))
		return
	}

	// Get file size
	info, err := os.Stat(tmpPath)
	if err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("failed to stat dump file: %v", err))
		return
	}
	backup.SizeBytes = info.Size()

	// Upload via object-storage provider (AWS SDK, no aws-cli shell-out).
	storage := s.providers.ObjectStorage(resource.Provider)
	if err := storage.Upload(ctx, resource.Config, tmpPath, fullS3Path); err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("S3 upload failed: %v", err))
		return
	}

	// Mark backup as completed
	now := time.Now()
	backup.Status = "completed"
	backup.FinishedAt = &now
	if err := s.store.SystemBackups().Update(ctx, backup); err != nil {
		s.logger.Error("failed to update backup record", slog.Any("error", err))
	}
	s.logger.Info("system backup completed", slog.String("file", fileName), slog.Int64("size", backup.SizeBytes))

	// Enforce retention: delete old backups beyond limit
	s.enforceRetention(ctx, cfg.Retention, resource, s3Path)
}

// failBackup marks a backup record as failed.
func (s *SystemBackupService) failBackup(ctx context.Context, backup *model.SystemBackup, errMsg string) {
	now := time.Now()
	backup.Status = "failed"
	backup.Error = errMsg
	backup.FinishedAt = &now
	if err := s.store.SystemBackups().Update(ctx, backup); err != nil {
		s.logger.Error("failed to update backup record", slog.Any("error", err))
	}
	s.logger.Error("system backup failed", slog.String("error", errMsg))
}

// enforceRetention deletes old backup records beyond the retention limit.
func (s *SystemBackupService) enforceRetention(ctx context.Context, retention int, resource *model.SharedResource, s3Path string) {
	if retention <= 0 {
		return
	}
	allBackups, total, err := s.store.SystemBackups().List(ctx, store.ListParams{Page: 1, PerPage: 1000})
	if err != nil || total <= retention {
		return
	}

	storage := s.providers.ObjectStorage(resource.Provider)

	// Backups are ordered by created_at DESC, so skip the first 'retention' entries
	for i := retention; i < len(allBackups); i++ {
		old := allBackups[i]
		if old.Status != "completed" {
			continue
		}
		if old.S3Path != "" && resource != nil {
			if err := storage.Delete(ctx, resource.Config, old.S3Path); err != nil {
				s.logger.Warn("failed to delete old backup from object storage",
					slog.String("path", old.S3Path), slog.Any("error", err))
			}
		}
		// Soft-delete the DB record
		now := time.Now()
		old.DeletedAt = &now
		if err := s.store.SystemBackups().Update(ctx, &old); err != nil {
			s.logger.Warn("failed to soft-delete old backup record", slog.String("id", old.ID.String()), slog.Any("error", err))
		}
	}
}

// StartScheduler runs a ticker loop to trigger scheduled backups.
// When isLeader is non-nil, only the elected leader triggers backups.
func (s *SystemBackupService) StartScheduler(ctx context.Context, isLeader func() bool) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !shouldRunSingleton(isLeader) {
					continue
				}
				cfg, err := s.GetConfig(context.Background())
				if err != nil || !cfg.Enabled || cfg.Schedule == "" {
					continue
				}
				if shouldRunNow(cfg.Schedule) {
					s.logger.Info("triggering scheduled system backup")
					if _, err := s.TriggerBackup(context.Background()); err != nil {
						s.logger.Error("scheduled system backup failed", slog.Any("error", err))
					}
				}
			}
		}
	}()
}

// shouldRunNow checks if a cron expression matches the current minute.
func shouldRunNow(cronExpr string) bool {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return false
	}
	now := time.Now()
	return matchField(parts[0], now.Minute()) &&
		matchField(parts[1], now.Hour()) &&
		matchField(parts[2], now.Day()) &&
		matchField(parts[3], int(now.Month())) &&
		matchWeekdayField(parts[4], int(now.Weekday()))
}

// matchWeekdayField matches a cron day-of-week field. Standard cron accepts both
// 0 and 7 for Sunday, but time.Weekday() only ever reports Sunday as 0, so a
// schedule written with weekday 7 would otherwise silently never match.
func matchWeekdayField(pattern string, weekday int) bool {
	if matchField(pattern, weekday) {
		return true
	}
	if weekday == 0 {
		return matchField(pattern, 7)
	}
	return false
}

// matchField matches a cron field that may be a comma-list of atoms, where each
// atom is "*", "*/step", "N", "lo-hi", or "lo-hi/step".
func matchField(pattern string, value int) bool {
	for _, atom := range strings.Split(pattern, ",") {
		if matchCronAtom(atom, value) {
			return true
		}
	}
	return false
}

func matchCronAtom(atom string, value int) bool {
	step := 1
	if i := strings.Index(atom, "/"); i != -1 {
		s, err := strconv.Atoi(atom[i+1:])
		if err != nil || s <= 0 {
			return false
		}
		step = s
		atom = atom[:i]
	}
	if atom == "*" {
		return value%step == 0
	}
	if i := strings.Index(atom, "-"); i != -1 {
		lo, err1 := strconv.Atoi(atom[:i])
		hi, err2 := strconv.Atoi(atom[i+1:])
		if err1 != nil || err2 != nil || lo > hi {
			return false
		}
		return value >= lo && value <= hi && (value-lo)%step == 0
	}
	n, err := strconv.Atoi(atom)
	if err != nil {
		return false
	}
	if step == 1 {
		return n == value
	}
	return value >= n && (value-n)%step == 0
}

// parseDatabaseURL extracts connection components from a PostgreSQL URL.
func parseDatabaseURL(dbURL string) (host, port, user, password, dbname string, err error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("invalid database URL: %w", err)
	}

	host = u.Hostname()
	port = u.Port()
	if port == "" {
		port = "5432"
	}
	user = u.User.Username()
	password, _ = u.User.Password()
	dbname = strings.TrimPrefix(u.Path, "/")

	return host, port, user, password, dbname, nil
}

// S3BackupFile represents a backup file found in S3.
type S3BackupFile struct {
	Key          string `json:"key"`
	FileName     string `json:"file_name"`
	SizeBytes    int64  `json:"size_bytes"`
	LastModified string `json:"last_modified"`
}

// ScanS3Backups connects to S3 with provided credentials and lists available .dump files.
// Only allowed on fresh installations (no users registered).
func (s *SystemBackupService) ScanS3Backups(ctx context.Context, s3 orchestrator.S3Config, path string) ([]S3BackupFile, error) {
	// Safety check: only allow scan on fresh installation (no users)
	count, err := s.store.Users().Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check user count: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("restore scan is only available on fresh installations")
	}

	if path == "" {
		path = "orkai-backups"
	}

	cfg, err := json.Marshal(s3)
	if err != nil {
		return nil, fmt.Errorf("marshal s3 config: %w", err)
	}

	objects, err := s.providers.ObjectStorage("aws_s3").List(ctx, cfg, path)
	if err != nil {
		return nil, fmt.Errorf("failed to list object storage: %w", err)
	}

	var files []S3BackupFile
	for _, obj := range objects {
		if !strings.HasSuffix(obj.FileName, ".dump") {
			continue
		}
		files = append(files, S3BackupFile{
			Key:          obj.Key,
			FileName:     obj.FileName,
			SizeBytes:    obj.SizeBytes,
			LastModified: obj.LastModified,
		})
	}
	return files, nil
}

// RestoreFromS3 downloads a backup from S3 and restores it via pg_restore.
func (s *SystemBackupService) RestoreFromS3(ctx context.Context, s3 orchestrator.S3Config, s3Key string) error {
	// Safety check: only allow restore on fresh installation (no users)
	count, err := s.store.Users().Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check user count: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("restore only available on fresh installation")
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "orkai-restore-*.dump")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	// Download from object storage
	cfg, err := json.Marshal(s3)
	if err != nil {
		return fmt.Errorf("marshal s3 config: %w", err)
	}
	if err := s.providers.ObjectStorage("aws_s3").Download(ctx, cfg, s3Key, tmpPath); err != nil {
		return fmt.Errorf("S3 download failed: %v", err)
	}

	// Parse database URL
	host, port, user, password, dbname, parseErr := parseDatabaseURL(s.dbURL)
	if parseErr != nil {
		return fmt.Errorf("failed to parse database URL: %w", parseErr)
	}

	// Run pg_restore
	restoreCmd := exec.CommandContext(ctx, "pg_restore",
		"--clean", "--if-exists",
		"--no-owner", "--no-privileges",
		"-h", host, "-p", port, "-U", user, "-d", dbname,
		tmpPath,
	)
	restoreCmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	if output, err := restoreCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pg_restore failed: %v — %s", err, string(output))
	}

	s.logger.Info("system restore completed", slog.String("s3_key", s3Key))
	return nil
}
