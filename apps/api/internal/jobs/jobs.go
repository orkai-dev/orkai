package jobs

import (
	"github.com/google/uuid"
)

const (
	QueueName = "orkai_jobs"
)

// JobType identifies the kind of background work to perform.
type JobType string

const (
	JobDeploy         JobType = "deploy"
	JobSystemBackup   JobType = "system_backup"
	JobDatabaseBackup JobType = "database_backup"
	JobPageDeploy     JobType = "page_deploy"
	JobWorkerDeploy   JobType = "worker_deploy"
)

// Job is the JSON envelope stored in PGMQ messages.
type Job struct {
	Type               JobType    `json:"type"`
	DeployID           *uuid.UUID `json:"deploy_id,omitempty"`
	BackupID           *uuid.UUID `json:"backup_id,omitempty"`
	DatabaseBackupID   *uuid.UUID `json:"database_backup_id,omitempty"`
	ForceBuild         bool       `json:"force_build,omitempty"`
	PageDeploymentID   *uuid.UUID `json:"page_deployment_id,omitempty"`
	WorkerDeploymentID *uuid.UUID `json:"worker_deployment_id,omitempty"`
}

// NewDeployJob returns a deploy job for the given deployment ID.
func NewDeployJob(deployID uuid.UUID, forceBuild bool) Job {
	return Job{Type: JobDeploy, DeployID: &deployID, ForceBuild: forceBuild}
}

// NewSystemBackupJob returns a system backup job for the given backup record ID.
func NewSystemBackupJob(backupID uuid.UUID) Job {
	return Job{Type: JobSystemBackup, BackupID: &backupID}
}

// NewDatabaseBackupJob returns a managed-database backup job for the given
// database_backups record ID.
func NewDatabaseBackupJob(backupID uuid.UUID) Job {
	return Job{Type: JobDatabaseBackup, DatabaseBackupID: &backupID}
}

// NewPageDeployJob returns a page deploy job for the given page deployment ID.
func NewPageDeployJob(deploymentID uuid.UUID) Job {
	return Job{Type: JobPageDeploy, PageDeploymentID: &deploymentID}
}

// NewWorkerDeployJob returns a worker deploy job for the given worker deployment ID.
func NewWorkerDeployJob(deploymentID uuid.UUID) Job {
	return Job{Type: JobWorkerDeploy, WorkerDeploymentID: &deploymentID}
}
