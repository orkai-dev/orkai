package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

type NotificationChannelType string

const (
	NotifyEmail      NotificationChannelType = "email"
	NotifyTelegram   NotificationChannelType = "telegram"
	NotifyDiscord    NotificationChannelType = "discord"
	NotifySlack      NotificationChannelType = "slack"
	NotifyGoogleChat NotificationChannelType = "google_chat"
)

// NotifyEvent represents a notification event type.
type NotifyEvent string

const (
	EventDeploySuccess   NotifyEvent = "deploy_success"
	EventDeployFailed    NotifyEvent = "deploy_failed"
	EventDeployCancelled NotifyEvent = "deploy_cancelled"
	EventBuildTimeout    NotifyEvent = "build_timeout"
	EventAppCrashed      NotifyEvent = "app_crashed"
	EventAppStopped      NotifyEvent = "app_stopped"
	EventAppRestarted    NotifyEvent = "app_restarted"
	EventBackupSuccess   NotifyEvent = "backup_success"
	EventBackupFailed    NotifyEvent = "backup_failed"
	EventNodeOffline     NotifyEvent = "node_not_ready"
	EventDiskPressure    NotifyEvent = "disk_pressure"
	EventCertExpiring    NotifyEvent = "cert_expiring"
	EventMemberJoined    NotifyEvent = "member_joined"
	EventMemberRemoved   NotifyEvent = "member_removed"
	EventAlertFired      NotifyEvent = "alert_fired"
	EventAlertResolved   NotifyEvent = "alert_resolved"
	EventDatabaseCreated NotifyEvent = "database_created"
	EventDatabaseDeleted NotifyEvent = "database_deleted"

	EventAppDeleted          NotifyEvent = "app_deleted"
	EventPageDeleted         NotifyEvent = "page_deleted"
	EventWorkerDeleted       NotifyEvent = "worker_deleted"
	EventCronJobDeleted      NotifyEvent = "cronjob_deleted"
	EventProjectDeleted      NotifyEvent = "project_deleted"
	EventDomainDeleted       NotifyEvent = "domain_deleted"
	EventResourceDeleted     NotifyEvent = "resource_deleted"
	EventNodeDeleted         NotifyEvent = "node_deleted"
	EventTeamDeleted         NotifyEvent = "team_deleted"
	EventTeamMemberRemoved   NotifyEvent = "team_member_removed"
	EventInvitationCancelled NotifyEvent = "invitation_cancelled"
	EventAPIKeyRevoked       NotifyEvent = "api_key_revoked"
	EventDNSRecordDeleted    NotifyEvent = "dns_record_deleted"
	EventPVCDeleted          NotifyEvent = "pvc_deleted"
)

// AllNotifyEvents returns all available event types (for future filtering UI).
func AllNotifyEvents() []NotifyEvent {
	return []NotifyEvent{
		EventDeploySuccess, EventDeployFailed, EventDeployCancelled, EventBuildTimeout,
		EventAppCrashed, EventAppStopped, EventAppRestarted,
		EventBackupSuccess, EventBackupFailed,
		EventNodeOffline, EventDiskPressure, EventCertExpiring,
		EventMemberJoined, EventMemberRemoved,
		EventAlertFired, EventAlertResolved,
		EventDatabaseCreated, EventDatabaseDeleted,
		EventAppDeleted, EventPageDeleted, EventWorkerDeleted,
		EventCronJobDeleted, EventProjectDeleted, EventDomainDeleted,
		EventResourceDeleted, EventNodeDeleted, EventTeamDeleted,
		EventTeamMemberRemoved, EventInvitationCancelled, EventAPIKeyRevoked,
		EventDNSRecordDeleted, EventPVCDeleted,
	}
}

// NotifyEventInfo describes a notification event for the settings UI.
type NotifyEventInfo struct {
	Key      NotifyEvent `json:"key"`
	Label    string      `json:"label"`
	Category string      `json:"category"`
}

// AllNotifyEventInfos returns all events with display metadata for the settings UI.
func AllNotifyEventInfos() []NotifyEventInfo {
	return []NotifyEventInfo{
		// Deployments
		{EventDeploySuccess, "Deploy success", "Deployments"},
		{EventDeployFailed, "Deploy failed", "Deployments"},
		{EventDeployCancelled, "Deploy cancelled", "Deployments"},
		{EventBuildTimeout, "Build timeout", "Deployments"},
		// Applications
		{EventAppCrashed, "App crashed", "Applications"},
		{EventAppStopped, "App stopped", "Applications"},
		{EventAppRestarted, "App restarted", "Applications"},
		{EventAppDeleted, "App deleted", "Applications"},
		// Pages & Workers
		{EventPageDeleted, "Page deleted", "Pages & Workers"},
		{EventWorkerDeleted, "Worker deleted", "Pages & Workers"},
		// Databases
		{EventDatabaseCreated, "Database created", "Databases"},
		{EventDatabaseDeleted, "Database deleted", "Databases"},
		// Cron jobs
		{EventCronJobDeleted, "Cron job deleted", "Cron jobs"},
		// Projects & Resources
		{EventProjectDeleted, "Project deleted", "Projects & Resources"},
		{EventDomainDeleted, "Domain deleted", "Projects & Resources"},
		{EventResourceDeleted, "Shared resource deleted", "Projects & Resources"},
		{EventDNSRecordDeleted, "DNS record deleted", "Projects & Resources"},
		{EventPVCDeleted, "PVC deleted", "Projects & Resources"},
		// Cluster
		{EventBackupSuccess, "Backup success", "Cluster"},
		{EventBackupFailed, "Backup failed", "Cluster"},
		{EventNodeOffline, "Node offline", "Cluster"},
		{EventDiskPressure, "Disk pressure", "Cluster"},
		{EventCertExpiring, "Certificate expiring", "Cluster"},
		{EventNodeDeleted, "Node deleted", "Cluster"},
		{EventAlertFired, "Alert fired", "Cluster"},
		{EventAlertResolved, "Alert resolved", "Cluster"},
		// Team
		{EventMemberJoined, "Member joined", "Team"},
		{EventMemberRemoved, "Member removed", "Team"},
		{EventTeamDeleted, "Team deleted", "Team"},
		{EventTeamMemberRemoved, "Team member removed", "Team"},
		{EventInvitationCancelled, "Invitation cancelled", "Team"},
		{EventAPIKeyRevoked, "API key revoked", "Team"},
	}
}

type NotificationChannel struct {
	BaseModel `bun:"table:notification_channels,alias:nc"`
	OrgID     uuid.UUID               `bun:"org_id,notnull,type:uuid" json:"org_id"`
	Type      NotificationChannelType `bun:"type,notnull" json:"type"`
	Enabled   bool                    `bun:"enabled,default:false" json:"enabled"`
	Config    json.RawMessage         `bun:"config,type:jsonb,default:'{}'" json:"config"`
}
