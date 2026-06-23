package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

// Store aggregates all repository interfaces.
type Store interface {
	// Tx runs fn inside a single database transaction. The closure receives a
	// transactional Store; all writes through it commit or roll back atomically.
	Tx(ctx context.Context, fn func(ctx context.Context, tx Store) error) error

	Organizations() OrganizationStore
	Users() UserStore
	Projects() ProjectStore
	Applications() ApplicationStore
	Deployments() DeploymentStore
	Domains() DomainStore
	ManagedDatabases() ManagedDatabaseStore
	Templates() TemplateStore
	Settings() SettingStore
	ServerNodes() ServerNodeStore
	SharedResources() SharedResourceStore
	CronJobs() CronJobStore
	CronJobRuns() CronJobRunStore
	DatabaseBackups() DatabaseBackupStore
	Teams() TeamStore
	TeamMembers() TeamMemberStore
	Invitations() InvitationStore
	NotificationChannels() NotificationChannelStore
	SystemBackups() SystemBackupStore
	Identities() IdentityStore
	OAuthChallenges() OAuthChallengeStore
	DeployTargets() DeployTargetStore
	Pages() PageStore
	PageDeployments() PageDeploymentStore
	Workers() WorkerStore
	WorkerDeployments() WorkerDeploymentStore
	APIKeys() APIKeyStore
}

// Pagination request parameters.
type ListParams struct {
	Page    int
	PerPage int
}

func (p ListParams) Offset() int {
	return (p.Page - 1) * p.PerPage
}

func (p ListParams) Limit() int {
	return p.PerPage
}

// DefaultListParams returns sensible defaults.
func DefaultListParams() ListParams {
	return ListParams{Page: 1, PerPage: 20}
}

// ============================================================================
// Repository Interfaces
// ============================================================================

type OrganizationStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error)
	Create(ctx context.Context, org *model.Organization) error
	Update(ctx context.Context, org *model.Organization) error
}

type UserStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	Update2FA(ctx context.Context, userID uuid.UUID, enabled bool, secret string) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, params ListParams) ([]model.User, int, error)
	UpdateRole(ctx context.Context, userID uuid.UUID, role string) error
	RemoveFromOrg(ctx context.Context, userID uuid.UUID) error
	BumpTokenVersion(ctx context.Context, userID uuid.UUID) error
	Count(ctx context.Context) (int, error)
	CountByRole(ctx context.Context, orgID uuid.UUID, role string) (int, error)
}

type ProjectStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	Create(ctx context.Context, project *model.Project) error
	Update(ctx context.Context, project *model.Project) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, params ListParams) ([]model.Project, int, error)
	ListByTeams(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID, params ListParams) ([]model.Project, int, error)
	// ListIDsByTeams returns only the project IDs owned by the given teams in the
	// org. Selecting just the id column avoids hydrating full project rows for
	// the access-scoping hot path.
	ListIDsByTeams(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID) ([]uuid.UUID, error)
}

// AppListFilter provides optional filters for global app queries.
type AppListFilter struct {
	Search     string      // name contains
	Status     string      // exact status match
	ProjectIDs []uuid.UUID // restrict to these projects (nil = all)
}

// PageListFilter provides optional filters for global page queries.
type PageListFilter struct {
	Search     string
	Status     string
	Provider   string // aws_cloudfront | cloudflare_pages
	ProjectIDs []uuid.UUID
}

// WorkerListFilter provides optional filters for global worker queries.
type WorkerListFilter struct {
	Search     string
	Status     string
	ProjectIDs []uuid.UUID
}

// DatabaseListFilter provides optional filters for global database queries.
type DatabaseListFilter struct {
	Search     string
	Status     string
	ProjectIDs []uuid.UUID
}

type ApplicationStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error)
	Create(ctx context.Context, app *model.Application) error
	Update(ctx context.Context, app *model.Application) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.AppStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.Application, int, error)
	ListAll(ctx context.Context, params ListParams, filter AppListFilter) ([]model.Application, int, error)
	// ListWithRegistry returns all applications that have a registry linked.
	ListWithRegistry(ctx context.Context) ([]model.Application, error)
	// ExistsByK8sName reports whether the project already has an application
	// with the given sanitized K8s name. Backed by the (project_id, k8s_name)
	// index so it stays O(index seek) instead of scanning every app.
	ExistsByK8sName(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error)
	// FindByResource returns the first application referencing resourceID as its
	// git provider or registry, or nil when none does. Used to guard shared
	// resource deletion without scanning every application.
	FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Application, error)
}

// DeploymentListFilter provides optional filters for global deployment queries.
type DeploymentListFilter struct {
	Status     string      // optional status filter (queued, building, deploying, success, failed, cancelled)
	ProjectIDs []uuid.UUID // restrict to these projects (nil = all)
}

type DeploymentStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error)
	Create(ctx context.Context, deploy *model.Deployment) error
	Update(ctx context.Context, deploy *model.Deployment) error
	ListByApp(ctx context.Context, appID uuid.UUID, params ListParams) ([]model.Deployment, int, error)
	ListAll(ctx context.Context, params ListParams, filter DeploymentListFilter) ([]model.Deployment, int, error)
	GetLatestByApp(ctx context.Context, appID uuid.UUID) (*model.Deployment, error)
}

type DomainStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Domain, error)
	Create(ctx context.Context, domain *model.Domain) error
	Update(ctx context.Context, domain *model.Domain) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByApp(ctx context.Context, appID uuid.UUID) ([]model.Domain, error)
	GetByHost(ctx context.Context, host string) (*model.Domain, error)
	// ListAllHosts returns every domain host across all apps in a single query.
	// Used for cluster orphan-ingress detection instead of an app-by-app N+1.
	ListAllHosts(ctx context.Context) ([]string, error)
}

type ManagedDatabaseStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error)
	Create(ctx context.Context, db *model.ManagedDatabase) error
	Update(ctx context.Context, db *model.ManagedDatabase) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.ManagedDatabase, int, error)
	ListAll(ctx context.Context, params ListParams, filter DatabaseListFilter) ([]model.ManagedDatabase, int, error)
	FindByExternalPort(ctx context.Context, port int32) (*model.ManagedDatabase, error)
	ListExternalPorts(ctx context.Context) ([]model.ExternalPortInfo, error)
	ListBackupEnabled(ctx context.Context) ([]model.ManagedDatabase, error)
	// ExistsByK8sName reports whether the project already has a managed database
	// with the given sanitized K8s name. Backed by the (project_id, k8s_name)
	// index.
	ExistsByK8sName(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error)
	// FindByBackupS3 returns the first managed database referencing s3ID as its
	// backup destination, or nil when none does.
	FindByBackupS3(ctx context.Context, s3ID uuid.UUID) (*model.ManagedDatabase, error)
}

type PageStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Page, error)
	Create(ctx context.Context, page *model.Page) error
	Update(ctx context.Context, page *model.Page) error
	// UpdateSettings writes only the user-editable settings columns, excluding
	// runtime and status, so a concurrent settings PATCH can't clobber the
	// runtime state the worker persists during a deploy.
	UpdateSettings(ctx context.Context, page *model.Page) error
	// UpdateSettingsIfNotDeploying is UpdateSettings with an atomic
	// "status <> deploying" guard. It returns false (without writing) when the
	// page is mid-deploy, closing the TOCTOU window where a settings PATCH that
	// repoints the cloud target could race a concurrent TryMarkDeploying. Used
	// for changes (cloud_account_id, region) that must not land while a deploy
	// is provisioning into the current target.
	UpdateSettingsIfNotDeploying(ctx context.Context, page *model.Page) (bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// DeleteIfNotDeploying deletes the page only when it isn't mid-deploy,
	// atomically. It returns the deleted row (so the caller can tear down its
	// cloud resources) or nil if nothing was deleted because a deploy is in
	// progress. Closes the TOCTOU window with TryMarkDeploying that would
	// otherwise orphan AWS resources.
	DeleteIfNotDeploying(ctx context.Context, id uuid.UUID) (*model.Page, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.Page, int, error)
	ListAll(ctx context.Context, params ListParams, filter PageListFilter) ([]model.Page, int, error)
	ExistsByName(ctx context.Context, projectID uuid.UUID, name string) (bool, error)
	// FindByResource returns the first page referencing resourceID as its cloud
	// account or git provider, or nil when none does.
	FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Page, error)
	TryMarkDeploying(ctx context.Context, id uuid.UUID) (bool, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.PageStatus) error
	UpdateRuntime(ctx context.Context, id uuid.UUID, rt *model.PageRuntime) error
}

type PageDeploymentStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.PageDeployment, error)
	Create(ctx context.Context, dep *model.PageDeployment) error
	Update(ctx context.Context, dep *model.PageDeployment) error
	ListByPage(ctx context.Context, pageID uuid.UUID, params ListParams) ([]model.PageDeployment, int, error)
	GetLatestByPage(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error)
	GetLatestSuccess(ctx context.Context, pageID uuid.UUID) (*model.PageDeployment, error)
	ListByStatus(ctx context.Context, status model.PageDeploymentStatus, params ListParams) ([]model.PageDeployment, int, error)
	// MarkTimedOut atomically transitions a deployment to "failed" (appending
	// logSuffix) only while it is still "deploying", returning whether a row was
	// updated. Used by stale recovery so it can't overwrite a deploy that
	// succeeded between the ListByStatus read and this write.
	MarkTimedOut(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error)
}

type WorkerStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Worker, error)
	Create(ctx context.Context, worker *model.Worker) error
	Update(ctx context.Context, worker *model.Worker) error
	// UpdateSettings writes only the user-editable settings columns, excluding
	// runtime and status, so a concurrent settings PATCH can't clobber the
	// runtime state the worker persists during a deploy.
	UpdateSettings(ctx context.Context, worker *model.Worker) error
	// UpdateSettingsIfNotDeploying is UpdateSettings with an atomic
	// "status <> deploying" guard. Returns false (without writing) when the
	// worker is mid-deploy.
	UpdateSettingsIfNotDeploying(ctx context.Context, worker *model.Worker) (bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// DeleteIfNotDeploying deletes the worker only when it isn't mid-deploy,
	// atomically, returning the deleted row (for cloud teardown) or nil.
	DeleteIfNotDeploying(ctx context.Context, id uuid.UUID) (*model.Worker, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.Worker, int, error)
	ListAll(ctx context.Context, params ListParams, filter WorkerListFilter) ([]model.Worker, int, error)
	ExistsByName(ctx context.Context, projectID uuid.UUID, name string) (bool, error)
	// FindByResource returns the first worker referencing resourceID as its
	// cloud account or git provider, or nil when none does.
	FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Worker, error)
	TryMarkDeploying(ctx context.Context, id uuid.UUID) (bool, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.WorkerStatus) error
	UpdateRuntime(ctx context.Context, id uuid.UUID, rt *model.WorkerRuntime) error
}

type WorkerDeploymentStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkerDeployment, error)
	Create(ctx context.Context, dep *model.WorkerDeployment) error
	Update(ctx context.Context, dep *model.WorkerDeployment) error
	ListByWorker(ctx context.Context, workerID uuid.UUID, params ListParams) ([]model.WorkerDeployment, int, error)
	GetLatestByWorker(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error)
	GetLatestSuccess(ctx context.Context, workerID uuid.UUID) (*model.WorkerDeployment, error)
	ListByStatus(ctx context.Context, status model.WorkerDeploymentStatus, params ListParams) ([]model.WorkerDeployment, int, error)
	// MarkTimedOut atomically transitions a deployment to "failed" (appending
	// logSuffix) only while it is still "deploying", returning whether a row was
	// updated.
	MarkTimedOut(ctx context.Context, id uuid.UUID, finishedAt time.Time, logSuffix string) (bool, error)
	// TryClaimNeedsConfirmation atomically retires a needs_confirmation deployment
	// so only one ConfirmR2 proceeds. Returns false when the deployment is no
	// longer awaiting confirmation.
	TryClaimNeedsConfirmation(ctx context.Context, id uuid.UUID) (bool, error)
}

type TemplateStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Template, error)
	GetByName(ctx context.Context, name string) (*model.Template, error)
	List(ctx context.Context, params ListParams) ([]model.Template, int, error)
	Create(ctx context.Context, tpl *model.Template) error
}

type SettingStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) ([]model.Setting, error)
}

type ServerNodeStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.ServerNode, error)
	Create(ctx context.Context, node *model.ServerNode) error
	Update(ctx context.Context, node *model.ServerNode) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]model.ServerNode, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.NodeStatus, msg string) error
	// FindBySSHKey returns the first node referencing sshKeyID, or nil when none
	// does. Used to guard SSH-key shared resource deletion.
	FindBySSHKey(ctx context.Context, sshKeyID uuid.UUID) (*model.ServerNode, error)
}

type SharedResourceStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.SharedResource, error)
	Create(ctx context.Context, resource *model.SharedResource) error
	Update(ctx context.Context, resource *model.SharedResource) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error)
}

type CronJobStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.CronJob, error)
	Create(ctx context.Context, cj *model.CronJob) error
	Update(ctx context.Context, cj *model.CronJob) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.CronJob, int, error)
}

type CronJobRunStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.CronJobRun, error)
	Create(ctx context.Context, run *model.CronJobRun) error
	Update(ctx context.Context, run *model.CronJobRun) error
	ListByCronJob(ctx context.Context, cronJobID uuid.UUID, params ListParams) ([]model.CronJobRun, int, error)
}

type DatabaseBackupStore interface {
	Create(ctx context.Context, backup *model.DatabaseBackup) error
	Update(ctx context.Context, backup *model.DatabaseBackup) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error)
	ListByDatabase(ctx context.Context, databaseID uuid.UUID, params ListParams) ([]model.DatabaseBackup, int, error)
}

type TeamStore interface {
	Create(ctx context.Context, team *model.Team) error
	Update(ctx context.Context, team *model.Team) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Team, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.Team, error)
	CountProjects(ctx context.Context, teamID uuid.UUID) (int, error)
}

type TeamMemberStore interface {
	Add(ctx context.Context, teamID, userID uuid.UUID) error
	Remove(ctx context.Context, teamID, userID uuid.UUID) error
	ListByTeam(ctx context.Context, teamID uuid.UUID) ([]model.TeamMember, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.TeamMember, error)
	ListTeamIDsByUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	ListUsersByTeam(ctx context.Context, teamID uuid.UUID) ([]model.OrgMember, error)
}

type InvitationStore interface {
	Create(ctx context.Context, inv *model.Invitation) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error)
	GetByToken(ctx context.Context, token string) (*model.Invitation, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, inv *model.Invitation) error
}

type NotificationChannelStore interface {
	GetByOrgAndType(ctx context.Context, orgID uuid.UUID, channelType string) (*model.NotificationChannel, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error)
	ListAllEnabled(ctx context.Context) ([]model.NotificationChannel, error)
	Upsert(ctx context.Context, channel *model.NotificationChannel) error
}

type SystemBackupStore interface {
	Create(ctx context.Context, backup *model.SystemBackup) error
	Update(ctx context.Context, backup *model.SystemBackup) error
	List(ctx context.Context, params ListParams) ([]model.SystemBackup, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.SystemBackup, error)
}

type IdentityStore interface {
	GetByProviderSubject(ctx context.Context, provider, subject string) (*model.UserIdentity, error)
	Create(ctx context.Context, identity *model.UserIdentity) error
}

// OAuthChallengeStore enforces single use of OAuth 2FA challenge tokens across
// API instances and restarts by persisting redeemed token IDs (jti).
type OAuthChallengeStore interface {
	// Consume atomically records jti as redeemed. It returns true on the first
	// redemption and false if the jti was already consumed (a replay).
	Consume(ctx context.Context, jti string, expiresAt time.Time) (bool, error)
}

type DeployTargetStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.DeployTarget, error)
	GetDefault(ctx context.Context) (*model.DeployTarget, error)
	List(ctx context.Context) ([]model.DeployTarget, error)
	Create(ctx context.Context, rec *model.DeployTarget) error
	Update(ctx context.Context, rec *model.DeployTarget) error
}

type APIKeyStore interface {
	Create(ctx context.Context, key *model.APIKey) error
	GetByHash(ctx context.Context, hash string) (*model.APIKey, error)
	GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*model.APIKey, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.APIKey, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	TouchLastUsed(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
}
