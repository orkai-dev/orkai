// Package testsupport provides shared fakes and helpers for unit tests across
// the apps/api module. It is intended to be imported only from _test.go files.
package testsupport

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// FakeStore is a configurable in-memory implementation of store.Store.
// Each sub-store exposes function fields (e.g. GetByIDFn) that tests can set to
// control behaviour. Unset functions return zero values and a nil error.
type FakeStore struct {
	OrganizationsStore       *FakeOrganizationStore
	UsersStore               *FakeUserStore
	ProjectsStore            *FakeProjectStore
	ApplicationsStore        *FakeApplicationStore
	DeploymentsStore         *FakeDeploymentStore
	DomainsStore             *FakeDomainStore
	ManagedDatabasesStore    *FakeManagedDatabaseStore
	TemplatesStore           *FakeTemplateStore
	SettingsStore            *FakeSettingStore
	ServerNodesStore         *FakeServerNodeStore
	SharedResourcesStore     *FakeSharedResourceStore
	CronJobsStore            *FakeCronJobStore
	CronJobRunsStore         *FakeCronJobRunStore
	DatabaseBackupsStore     *FakeDatabaseBackupStore
	TeamsStore               *FakeTeamStore
	TeamMembersStore         *FakeTeamMemberStore
	InvitationsStore         *FakeInvitationStore
	NotificationChannelStore *FakeNotificationChannelStore
	SystemBackupsStore       *FakeSystemBackupStore
	IdentitiesStore          *FakeIdentityStore
	OAuthChallengesStore     *FakeOAuthChallengeStore
	DeployTargetsStore       *FakeDeployTargetStore
	PagesStore               *FakePageStore
	PageDeploymentsStore     *FakePageDeploymentStore
	WorkersStore             *FakeWorkerStore
	WorkerDeploymentsStore   *FakeWorkerDeploymentStore
	APIKeysStore             *FakeAPIKeyStore
	TxFn                     func(ctx context.Context, fn func(ctx context.Context, tx store.Store) error) error
}

// NewFakeStore returns a FakeStore with all sub-stores initialised.
func NewFakeStore() *FakeStore {
	return &FakeStore{
		OrganizationsStore:       &FakeOrganizationStore{},
		UsersStore:               &FakeUserStore{},
		ProjectsStore:            &FakeProjectStore{},
		ApplicationsStore:        &FakeApplicationStore{},
		DeploymentsStore:         &FakeDeploymentStore{},
		DomainsStore:             &FakeDomainStore{},
		ManagedDatabasesStore:    &FakeManagedDatabaseStore{},
		TemplatesStore:           &FakeTemplateStore{},
		SettingsStore:            &FakeSettingStore{},
		ServerNodesStore:         &FakeServerNodeStore{},
		SharedResourcesStore:     &FakeSharedResourceStore{},
		CronJobsStore:            &FakeCronJobStore{},
		CronJobRunsStore:         &FakeCronJobRunStore{},
		DatabaseBackupsStore:     &FakeDatabaseBackupStore{},
		TeamsStore:               &FakeTeamStore{},
		TeamMembersStore:         &FakeTeamMemberStore{},
		InvitationsStore:         &FakeInvitationStore{},
		NotificationChannelStore: &FakeNotificationChannelStore{},
		SystemBackupsStore:       &FakeSystemBackupStore{},
		IdentitiesStore:          &FakeIdentityStore{},
		OAuthChallengesStore:     &FakeOAuthChallengeStore{},
		DeployTargetsStore:       &FakeDeployTargetStore{},
		PagesStore:               &FakePageStore{},
		PageDeploymentsStore:     &FakePageDeploymentStore{},
		WorkersStore:             &FakeWorkerStore{},
		WorkerDeploymentsStore:   &FakeWorkerDeploymentStore{},
		APIKeysStore:             &FakeAPIKeyStore{},
	}
}

var _ store.Store = (*FakeStore)(nil)

func (f *FakeStore) Tx(ctx context.Context, fn func(ctx context.Context, tx store.Store) error) error {
	if f.TxFn != nil {
		return f.TxFn(ctx, fn)
	}
	return fn(ctx, f)
}

func (f *FakeStore) Organizations() store.OrganizationStore { return f.OrganizationsStore }
func (f *FakeStore) Users() store.UserStore                 { return f.UsersStore }
func (f *FakeStore) Projects() store.ProjectStore           { return f.ProjectsStore }
func (f *FakeStore) Applications() store.ApplicationStore   { return f.ApplicationsStore }
func (f *FakeStore) Deployments() store.DeploymentStore     { return f.DeploymentsStore }
func (f *FakeStore) Domains() store.DomainStore             { return f.DomainsStore }
func (f *FakeStore) ManagedDatabases() store.ManagedDatabaseStore {
	return f.ManagedDatabasesStore
}
func (f *FakeStore) Templates() store.TemplateStore     { return f.TemplatesStore }
func (f *FakeStore) Settings() store.SettingStore       { return f.SettingsStore }
func (f *FakeStore) ServerNodes() store.ServerNodeStore { return f.ServerNodesStore }
func (f *FakeStore) SharedResources() store.SharedResourceStore {
	return f.SharedResourcesStore
}
func (f *FakeStore) CronJobs() store.CronJobStore       { return f.CronJobsStore }
func (f *FakeStore) CronJobRuns() store.CronJobRunStore { return f.CronJobRunsStore }
func (f *FakeStore) DatabaseBackups() store.DatabaseBackupStore {
	return f.DatabaseBackupsStore
}
func (f *FakeStore) Teams() store.TeamStore             { return f.TeamsStore }
func (f *FakeStore) TeamMembers() store.TeamMemberStore { return f.TeamMembersStore }
func (f *FakeStore) Invitations() store.InvitationStore { return f.InvitationsStore }
func (f *FakeStore) NotificationChannels() store.NotificationChannelStore {
	return f.NotificationChannelStore
}
func (f *FakeStore) SystemBackups() store.SystemBackupStore { return f.SystemBackupsStore }
func (f *FakeStore) Identities() store.IdentityStore        { return f.IdentitiesStore }
func (f *FakeStore) OAuthChallenges() store.OAuthChallengeStore {
	return f.OAuthChallengesStore
}
func (f *FakeStore) DeployTargets() store.DeployTargetStore { return f.DeployTargetsStore }
func (f *FakeStore) Pages() store.PageStore                 { return f.PagesStore }
func (f *FakeStore) PageDeployments() store.PageDeploymentStore {
	return f.PageDeploymentsStore
}
func (f *FakeStore) Workers() store.WorkerStore { return f.WorkersStore }
func (f *FakeStore) WorkerDeployments() store.WorkerDeploymentStore {
	return f.WorkerDeploymentsStore
}
func (f *FakeStore) APIKeys() store.APIKeyStore { return f.APIKeysStore }

// ─── APIKeyStore ─────────────────────────────────────────────────

type FakeAPIKeyStore struct {
	CreateFn           func(ctx context.Context, key *model.APIKey) error
	GetByHashFn        func(ctx context.Context, hash string) (*model.APIKey, error)
	GetByIDForUserFn   func(ctx context.Context, id, userID uuid.UUID) (*model.APIKey, error)
	ListByUserFn       func(ctx context.Context, userID uuid.UUID) ([]model.APIKey, error)
	DeleteFn           func(ctx context.Context, id, userID uuid.UUID) error
	TouchLastUsedFn    func(ctx context.Context, id uuid.UUID) error
	RevokeAllForUserFn func(ctx context.Context, userID uuid.UUID) error
}

func (f *FakeAPIKeyStore) Create(ctx context.Context, key *model.APIKey) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, key)
	}
	return nil
}

func (f *FakeAPIKeyStore) GetByHash(ctx context.Context, hash string) (*model.APIKey, error) {
	if f.GetByHashFn != nil {
		return f.GetByHashFn(ctx, hash)
	}
	return nil, sql.ErrNoRows
}

func (f *FakeAPIKeyStore) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*model.APIKey, error) {
	if f.GetByIDForUserFn != nil {
		return f.GetByIDForUserFn(ctx, id, userID)
	}
	return nil, sql.ErrNoRows
}

func (f *FakeAPIKeyStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.APIKey, error) {
	if f.ListByUserFn != nil {
		return f.ListByUserFn(ctx, userID)
	}
	return nil, nil
}

func (f *FakeAPIKeyStore) Delete(ctx context.Context, id, userID uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id, userID)
	}
	return nil
}

func (f *FakeAPIKeyStore) TouchLastUsed(ctx context.Context, id uuid.UUID) error {
	if f.TouchLastUsedFn != nil {
		return f.TouchLastUsedFn(ctx, id)
	}
	return nil
}

func (f *FakeAPIKeyStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	if f.RevokeAllForUserFn != nil {
		return f.RevokeAllForUserFn(ctx, userID)
	}
	return nil
}

// ─── DeployTargetStore ───────────────────────────────────────────

type FakeDeployTargetStore struct {
	GetByIDFn    func(ctx context.Context, id uuid.UUID) (*model.DeployTarget, error)
	GetDefaultFn func(ctx context.Context) (*model.DeployTarget, error)
	ListFn       func(ctx context.Context) ([]model.DeployTarget, error)
	CreateFn     func(ctx context.Context, rec *model.DeployTarget) error
	UpdateFn     func(ctx context.Context, rec *model.DeployTarget) error

	records   map[uuid.UUID]*model.DeployTarget
	defaultID uuid.UUID
}

func (f *FakeDeployTargetStore) seed() {
	if f.records == nil {
		f.records = map[uuid.UUID]*model.DeployTarget{
			model.DefaultDeployTargetID: {
				BaseModel:    model.BaseModel{ID: model.DefaultDeployTargetID},
				Kind:         model.DeployTargetK3s,
				Capabilities: []string{"deploy", "exec", "kubernetes"},
				IsDefault:    true,
			},
		}
		f.defaultID = model.DefaultDeployTargetID
	}
}

func (f *FakeDeployTargetStore) GetByID(ctx context.Context, id uuid.UUID) (*model.DeployTarget, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	f.seed()
	rec, ok := f.records[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return rec, nil
}

func (f *FakeDeployTargetStore) GetDefault(ctx context.Context) (*model.DeployTarget, error) {
	if f.GetDefaultFn != nil {
		return f.GetDefaultFn(ctx)
	}
	f.seed()
	return f.records[f.defaultID], nil
}

func (f *FakeDeployTargetStore) List(ctx context.Context) ([]model.DeployTarget, error) {
	if f.ListFn != nil {
		return f.ListFn(ctx)
	}
	f.seed()
	out := make([]model.DeployTarget, 0, len(f.records))
	for _, r := range f.records {
		out = append(out, *r)
	}
	return out, nil
}

func (f *FakeDeployTargetStore) Create(ctx context.Context, rec *model.DeployTarget) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, rec)
	}
	f.seed()
	f.records[rec.ID] = rec
	return nil
}

func (f *FakeDeployTargetStore) Update(ctx context.Context, rec *model.DeployTarget) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, rec)
	}
	f.seed()
	f.records[rec.ID] = rec
	return nil
}

// ─── OAuthChallengeStore ─────────────────────────────────────────

// FakeOAuthChallengeStore tracks consumed challenge IDs in memory. When no
// ConsumeFn is set it enforces single use against its own map, which mirrors the
// real store's behaviour for tests.
type FakeOAuthChallengeStore struct {
	ConsumeFn func(ctx context.Context, jti string, expiresAt time.Time) (bool, error)

	consumed map[string]struct{}
}

func (f *FakeOAuthChallengeStore) Consume(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	if f.ConsumeFn != nil {
		return f.ConsumeFn(ctx, jti, expiresAt)
	}
	if f.consumed == nil {
		f.consumed = make(map[string]struct{})
	}
	if _, used := f.consumed[jti]; used {
		return false, nil
	}
	f.consumed[jti] = struct{}{}
	return true, nil
}

// ─── IdentityStore ───────────────────────────────────────────────

type FakeIdentityStore struct {
	GetByProviderSubjectFn func(ctx context.Context, provider, subject string) (*model.UserIdentity, error)
	CreateFn               func(ctx context.Context, identity *model.UserIdentity) error
}

func (f *FakeIdentityStore) GetByProviderSubject(ctx context.Context, provider, subject string) (*model.UserIdentity, error) {
	if f.GetByProviderSubjectFn != nil {
		return f.GetByProviderSubjectFn(ctx, provider, subject)
	}
	return nil, errors.New("not found")
}

func (f *FakeIdentityStore) Create(ctx context.Context, identity *model.UserIdentity) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, identity)
	}
	return nil
}

// ─── OrganizationStore ───────────────────────────────────────────

type FakeOrganizationStore struct {
	GetByIDFn func(ctx context.Context, id uuid.UUID) (*model.Organization, error)
	CreateFn  func(ctx context.Context, org *model.Organization) error
	UpdateFn  func(ctx context.Context, org *model.Organization) error
}

func (f *FakeOrganizationStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeOrganizationStore) Create(ctx context.Context, org *model.Organization) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, org)
	}
	return nil
}

func (f *FakeOrganizationStore) Update(ctx context.Context, org *model.Organization) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, org)
	}
	return nil
}

// ─── UserStore ───────────────────────────────────────────────────

type FakeUserStore struct {
	GetByIDFn          func(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmailFn       func(ctx context.Context, email string) (*model.User, error)
	CreateFn           func(ctx context.Context, user *model.User) error
	UpdateFn           func(ctx context.Context, user *model.User) error
	UpdatePasswordFn   func(ctx context.Context, userID uuid.UUID, passwordHash string) error
	Update2FAFn        func(ctx context.Context, userID uuid.UUID, enabled bool, secret string) error
	ListByOrgFn        func(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.User, int, error)
	UpdateRoleFn       func(ctx context.Context, userID uuid.UUID, role string) error
	RemoveFromOrgFn    func(ctx context.Context, userID uuid.UUID) error
	BumpTokenVersionFn func(ctx context.Context, userID uuid.UUID) error
	CountFn            func(ctx context.Context) (int, error)
	CountByRoleFn      func(ctx context.Context, orgID uuid.UUID, role string) (int, error)
}

func (f *FakeUserStore) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.GetByEmailFn != nil {
		return f.GetByEmailFn(ctx, email)
	}
	return nil, nil
}

func (f *FakeUserStore) Create(ctx context.Context, user *model.User) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, user)
	}
	return nil
}

func (f *FakeUserStore) Update(ctx context.Context, user *model.User) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, user)
	}
	return nil
}

func (f *FakeUserStore) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	if f.UpdatePasswordFn != nil {
		return f.UpdatePasswordFn(ctx, userID, passwordHash)
	}
	return nil
}

func (f *FakeUserStore) Update2FA(ctx context.Context, userID uuid.UUID, enabled bool, secret string) error {
	if f.Update2FAFn != nil {
		return f.Update2FAFn(ctx, userID, enabled, secret)
	}
	return nil
}

func (f *FakeUserStore) ListByOrg(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.User, int, error) {
	if f.ListByOrgFn != nil {
		return f.ListByOrgFn(ctx, orgID, params)
	}
	return nil, 0, nil
}

func (f *FakeUserStore) UpdateRole(ctx context.Context, userID uuid.UUID, role string) error {
	if f.UpdateRoleFn != nil {
		return f.UpdateRoleFn(ctx, userID, role)
	}
	return nil
}

func (f *FakeUserStore) RemoveFromOrg(ctx context.Context, userID uuid.UUID) error {
	if f.RemoveFromOrgFn != nil {
		return f.RemoveFromOrgFn(ctx, userID)
	}
	return nil
}

func (f *FakeUserStore) BumpTokenVersion(ctx context.Context, userID uuid.UUID) error {
	if f.BumpTokenVersionFn != nil {
		return f.BumpTokenVersionFn(ctx, userID)
	}
	return nil
}

func (f *FakeUserStore) Count(ctx context.Context) (int, error) {
	if f.CountFn != nil {
		return f.CountFn(ctx)
	}
	return 0, nil
}

func (f *FakeUserStore) CountByRole(ctx context.Context, orgID uuid.UUID, role string) (int, error) {
	if f.CountByRoleFn != nil {
		return f.CountByRoleFn(ctx, orgID, role)
	}
	return 0, nil
}

// ─── ProjectStore ────────────────────────────────────────────────

type FakeProjectStore struct {
	GetByIDFn        func(ctx context.Context, id uuid.UUID) (*model.Project, error)
	CreateFn         func(ctx context.Context, project *model.Project) error
	UpdateFn         func(ctx context.Context, project *model.Project) error
	DeleteFn         func(ctx context.Context, id uuid.UUID) error
	ListByOrgFn      func(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error)
	ListByTeamsFn    func(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID, params store.ListParams) ([]model.Project, int, error)
	ListIDsByTeamsFn func(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID) ([]uuid.UUID, error)
}

func (f *FakeProjectStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeProjectStore) Create(ctx context.Context, project *model.Project) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, project)
	}
	return nil
}

func (f *FakeProjectStore) Update(ctx context.Context, project *model.Project) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, project)
	}
	return nil
}

func (f *FakeProjectStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeProjectStore) ListByOrg(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
	if f.ListByOrgFn != nil {
		return f.ListByOrgFn(ctx, orgID, params)
	}
	return nil, 0, nil
}

func (f *FakeProjectStore) ListByTeams(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
	if f.ListByTeamsFn != nil {
		return f.ListByTeamsFn(ctx, orgID, teamIDs, params)
	}
	return nil, 0, nil
}

func (f *FakeProjectStore) ListIDsByTeams(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID) ([]uuid.UUID, error) {
	if f.ListIDsByTeamsFn != nil {
		return f.ListIDsByTeamsFn(ctx, orgID, teamIDs)
	}
	return nil, nil
}

// ─── ApplicationStore ────────────────────────────────────────────

type FakeApplicationStore struct {
	GetByIDFn          func(ctx context.Context, id uuid.UUID) (*model.Application, error)
	CreateFn           func(ctx context.Context, app *model.Application) error
	UpdateFn           func(ctx context.Context, app *model.Application) error
	UpdateStatusFn     func(ctx context.Context, id uuid.UUID, status model.AppStatus) error
	DeleteFn           func(ctx context.Context, id uuid.UUID) error
	ListByProjectFn    func(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Application, int, error)
	ListAllFn          func(ctx context.Context, params store.ListParams, filter store.AppListFilter) ([]model.Application, int, error)
	ListWithRegistryFn func(ctx context.Context) ([]model.Application, error)
	ExistsByK8sNameFn  func(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error)
	FindByResourceFn   func(ctx context.Context, resourceID uuid.UUID) (*model.Application, error)
}

func (f *FakeApplicationStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeApplicationStore) Create(ctx context.Context, app *model.Application) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, app)
	}
	return nil
}

func (f *FakeApplicationStore) Update(ctx context.Context, app *model.Application) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, app)
	}
	return nil
}

func (f *FakeApplicationStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.AppStatus) error {
	if f.UpdateStatusFn != nil {
		return f.UpdateStatusFn(ctx, id, status)
	}
	return nil
}

func (f *FakeApplicationStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeApplicationStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Application, int, error) {
	if f.ListByProjectFn != nil {
		return f.ListByProjectFn(ctx, projectID, params)
	}
	return nil, 0, nil
}

func (f *FakeApplicationStore) ListAll(ctx context.Context, params store.ListParams, filter store.AppListFilter) ([]model.Application, int, error) {
	if f.ListAllFn != nil {
		return f.ListAllFn(ctx, params, filter)
	}
	return nil, 0, nil
}

func (f *FakeApplicationStore) ListWithRegistry(ctx context.Context) ([]model.Application, error) {
	if f.ListWithRegistryFn != nil {
		return f.ListWithRegistryFn(ctx)
	}
	return nil, nil
}

func (f *FakeApplicationStore) ExistsByK8sName(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error) {
	if f.ExistsByK8sNameFn != nil {
		return f.ExistsByK8sNameFn(ctx, projectID, k8sName)
	}
	return false, nil
}

func (f *FakeApplicationStore) FindByResource(ctx context.Context, resourceID uuid.UUID) (*model.Application, error) {
	if f.FindByResourceFn != nil {
		return f.FindByResourceFn(ctx, resourceID)
	}
	return nil, nil
}

// ─── DeploymentStore ─────────────────────────────────────────────

type FakeDeploymentStore struct {
	GetByIDFn        func(ctx context.Context, id uuid.UUID) (*model.Deployment, error)
	CreateFn         func(ctx context.Context, deploy *model.Deployment) error
	UpdateFn         func(ctx context.Context, deploy *model.Deployment) error
	ListByAppFn      func(ctx context.Context, appID uuid.UUID, params store.ListParams) ([]model.Deployment, int, error)
	ListAllFn        func(ctx context.Context, params store.ListParams, filter store.DeploymentListFilter) ([]model.Deployment, int, error)
	GetLatestByAppFn func(ctx context.Context, appID uuid.UUID) (*model.Deployment, error)
}

func (f *FakeDeploymentStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeDeploymentStore) Create(ctx context.Context, deploy *model.Deployment) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, deploy)
	}
	return nil
}

func (f *FakeDeploymentStore) Update(ctx context.Context, deploy *model.Deployment) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, deploy)
	}
	return nil
}

func (f *FakeDeploymentStore) ListByApp(ctx context.Context, appID uuid.UUID, params store.ListParams) ([]model.Deployment, int, error) {
	if f.ListByAppFn != nil {
		return f.ListByAppFn(ctx, appID, params)
	}
	return nil, 0, nil
}

func (f *FakeDeploymentStore) ListAll(ctx context.Context, params store.ListParams, filter store.DeploymentListFilter) ([]model.Deployment, int, error) {
	if f.ListAllFn != nil {
		return f.ListAllFn(ctx, params, filter)
	}
	return nil, 0, nil
}

func (f *FakeDeploymentStore) GetLatestByApp(ctx context.Context, appID uuid.UUID) (*model.Deployment, error) {
	if f.GetLatestByAppFn != nil {
		return f.GetLatestByAppFn(ctx, appID)
	}
	return nil, nil
}

// ─── DomainStore ─────────────────────────────────────────────────

type FakeDomainStore struct {
	GetByIDFn      func(ctx context.Context, id uuid.UUID) (*model.Domain, error)
	CreateFn       func(ctx context.Context, domain *model.Domain) error
	UpdateFn       func(ctx context.Context, domain *model.Domain) error
	DeleteFn       func(ctx context.Context, id uuid.UUID) error
	ListByAppFn    func(ctx context.Context, appID uuid.UUID) ([]model.Domain, error)
	GetByHostFn    func(ctx context.Context, host string) (*model.Domain, error)
	ListAllHostsFn func(ctx context.Context) ([]string, error)
}

func (f *FakeDomainStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeDomainStore) Create(ctx context.Context, domain *model.Domain) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, domain)
	}
	return nil
}

func (f *FakeDomainStore) Update(ctx context.Context, domain *model.Domain) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, domain)
	}
	return nil
}

func (f *FakeDomainStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeDomainStore) ListByApp(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
	if f.ListByAppFn != nil {
		return f.ListByAppFn(ctx, appID)
	}
	return nil, nil
}

func (f *FakeDomainStore) GetByHost(ctx context.Context, host string) (*model.Domain, error) {
	if f.GetByHostFn != nil {
		return f.GetByHostFn(ctx, host)
	}
	return nil, nil
}

func (f *FakeDomainStore) ListAllHosts(ctx context.Context) ([]string, error) {
	if f.ListAllHostsFn != nil {
		return f.ListAllHostsFn(ctx)
	}
	return nil, nil
}

// ─── ManagedDatabaseStore ────────────────────────────────────────

type FakeManagedDatabaseStore struct {
	GetByIDFn            func(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error)
	CreateFn             func(ctx context.Context, db *model.ManagedDatabase) error
	UpdateFn             func(ctx context.Context, db *model.ManagedDatabase) error
	DeleteFn             func(ctx context.Context, id uuid.UUID) error
	ListByProjectFn      func(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.ManagedDatabase, int, error)
	ListAllFn            func(ctx context.Context, params store.ListParams, filter store.DatabaseListFilter) ([]model.ManagedDatabase, int, error)
	FindByExternalPortFn func(ctx context.Context, port int32) (*model.ManagedDatabase, error)
	ListExternalPortsFn  func(ctx context.Context) ([]model.ExternalPortInfo, error)
	ListBackupEnabledFn  func(ctx context.Context) ([]model.ManagedDatabase, error)
	ExistsByK8sNameFn    func(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error)
	FindByBackupS3Fn     func(ctx context.Context, s3ID uuid.UUID) (*model.ManagedDatabase, error)
}

func (f *FakeManagedDatabaseStore) GetByID(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeManagedDatabaseStore) Create(ctx context.Context, db *model.ManagedDatabase) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, db)
	}
	return nil
}

func (f *FakeManagedDatabaseStore) Update(ctx context.Context, db *model.ManagedDatabase) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, db)
	}
	return nil
}

func (f *FakeManagedDatabaseStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeManagedDatabaseStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.ManagedDatabase, int, error) {
	if f.ListByProjectFn != nil {
		return f.ListByProjectFn(ctx, projectID, params)
	}
	return nil, 0, nil
}

func (f *FakeManagedDatabaseStore) ListAll(ctx context.Context, params store.ListParams, filter store.DatabaseListFilter) ([]model.ManagedDatabase, int, error) {
	if f.ListAllFn != nil {
		return f.ListAllFn(ctx, params, filter)
	}
	return nil, 0, nil
}

func (f *FakeManagedDatabaseStore) FindByExternalPort(ctx context.Context, port int32) (*model.ManagedDatabase, error) {
	if f.FindByExternalPortFn != nil {
		return f.FindByExternalPortFn(ctx, port)
	}
	return nil, nil
}

func (f *FakeManagedDatabaseStore) ListExternalPorts(ctx context.Context) ([]model.ExternalPortInfo, error) {
	if f.ListExternalPortsFn != nil {
		return f.ListExternalPortsFn(ctx)
	}
	return nil, nil
}

func (f *FakeManagedDatabaseStore) ListBackupEnabled(ctx context.Context) ([]model.ManagedDatabase, error) {
	if f.ListBackupEnabledFn != nil {
		return f.ListBackupEnabledFn(ctx)
	}
	return nil, nil
}

func (f *FakeManagedDatabaseStore) ExistsByK8sName(ctx context.Context, projectID uuid.UUID, k8sName string) (bool, error) {
	if f.ExistsByK8sNameFn != nil {
		return f.ExistsByK8sNameFn(ctx, projectID, k8sName)
	}
	return false, nil
}

func (f *FakeManagedDatabaseStore) FindByBackupS3(ctx context.Context, s3ID uuid.UUID) (*model.ManagedDatabase, error) {
	if f.FindByBackupS3Fn != nil {
		return f.FindByBackupS3Fn(ctx, s3ID)
	}
	return nil, nil
}

// ─── TemplateStore ───────────────────────────────────────────────

type FakeTemplateStore struct {
	GetByIDFn   func(ctx context.Context, id uuid.UUID) (*model.Template, error)
	GetByNameFn func(ctx context.Context, name string) (*model.Template, error)
	ListFn      func(ctx context.Context, params store.ListParams) ([]model.Template, int, error)
	CreateFn    func(ctx context.Context, tpl *model.Template) error
}

func (f *FakeTemplateStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Template, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeTemplateStore) GetByName(ctx context.Context, name string) (*model.Template, error) {
	if f.GetByNameFn != nil {
		return f.GetByNameFn(ctx, name)
	}
	return nil, nil
}

func (f *FakeTemplateStore) List(ctx context.Context, params store.ListParams) ([]model.Template, int, error) {
	if f.ListFn != nil {
		return f.ListFn(ctx, params)
	}
	return nil, 0, nil
}

func (f *FakeTemplateStore) Create(ctx context.Context, tpl *model.Template) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, tpl)
	}
	return nil
}

// ─── SettingStore ────────────────────────────────────────────────

type FakeSettingStore struct {
	GetFn    func(ctx context.Context, key string) (string, error)
	SetFn    func(ctx context.Context, key, value string) error
	GetAllFn func(ctx context.Context) ([]model.Setting, error)
}

func (f *FakeSettingStore) Get(ctx context.Context, key string) (string, error) {
	if f.GetFn != nil {
		return f.GetFn(ctx, key)
	}
	return "", nil
}

func (f *FakeSettingStore) Set(ctx context.Context, key, value string) error {
	if f.SetFn != nil {
		return f.SetFn(ctx, key, value)
	}
	return nil
}

func (f *FakeSettingStore) GetAll(ctx context.Context) ([]model.Setting, error) {
	if f.GetAllFn != nil {
		return f.GetAllFn(ctx)
	}
	return nil, nil
}

// ─── ServerNodeStore ─────────────────────────────────────────────

type FakeServerNodeStore struct {
	GetByIDFn      func(ctx context.Context, id uuid.UUID) (*model.ServerNode, error)
	CreateFn       func(ctx context.Context, node *model.ServerNode) error
	UpdateFn       func(ctx context.Context, node *model.ServerNode) error
	DeleteFn       func(ctx context.Context, id uuid.UUID) error
	ListFn         func(ctx context.Context) ([]model.ServerNode, error)
	UpdateStatusFn func(ctx context.Context, id uuid.UUID, status model.NodeStatus, msg string) error
	FindBySSHKeyFn func(ctx context.Context, sshKeyID uuid.UUID) (*model.ServerNode, error)
}

func (f *FakeServerNodeStore) GetByID(ctx context.Context, id uuid.UUID) (*model.ServerNode, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeServerNodeStore) Create(ctx context.Context, node *model.ServerNode) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, node)
	}
	return nil
}

func (f *FakeServerNodeStore) Update(ctx context.Context, node *model.ServerNode) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, node)
	}
	return nil
}

func (f *FakeServerNodeStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeServerNodeStore) List(ctx context.Context) ([]model.ServerNode, error) {
	if f.ListFn != nil {
		return f.ListFn(ctx)
	}
	return nil, nil
}

func (f *FakeServerNodeStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.NodeStatus, msg string) error {
	if f.UpdateStatusFn != nil {
		return f.UpdateStatusFn(ctx, id, status, msg)
	}
	return nil
}

func (f *FakeServerNodeStore) FindBySSHKey(ctx context.Context, sshKeyID uuid.UUID) (*model.ServerNode, error) {
	if f.FindBySSHKeyFn != nil {
		return f.FindBySSHKeyFn(ctx, sshKeyID)
	}
	return nil, nil
}

// ─── SharedResourceStore ─────────────────────────────────────────

type FakeSharedResourceStore struct {
	GetByIDFn   func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error)
	CreateFn    func(ctx context.Context, resource *model.SharedResource) error
	UpdateFn    func(ctx context.Context, resource *model.SharedResource) error
	DeleteFn    func(ctx context.Context, id uuid.UUID) error
	ListByOrgFn func(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error)
}

func (f *FakeSharedResourceStore) GetByID(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeSharedResourceStore) Create(ctx context.Context, resource *model.SharedResource) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, resource)
	}
	return nil
}

func (f *FakeSharedResourceStore) Update(ctx context.Context, resource *model.SharedResource) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, resource)
	}
	return nil
}

func (f *FakeSharedResourceStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeSharedResourceStore) ListByOrg(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error) {
	if f.ListByOrgFn != nil {
		return f.ListByOrgFn(ctx, orgID, resourceType)
	}
	return nil, nil
}

// ─── CronJobStore ────────────────────────────────────────────────

type FakeCronJobStore struct {
	GetByIDFn       func(ctx context.Context, id uuid.UUID) (*model.CronJob, error)
	CreateFn        func(ctx context.Context, cj *model.CronJob) error
	UpdateFn        func(ctx context.Context, cj *model.CronJob) error
	DeleteFn        func(ctx context.Context, id uuid.UUID) error
	ListByProjectFn func(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.CronJob, int, error)
}

func (f *FakeCronJobStore) GetByID(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeCronJobStore) Create(ctx context.Context, cj *model.CronJob) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, cj)
	}
	return nil
}

func (f *FakeCronJobStore) Update(ctx context.Context, cj *model.CronJob) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, cj)
	}
	return nil
}

func (f *FakeCronJobStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeCronJobStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.CronJob, int, error) {
	if f.ListByProjectFn != nil {
		return f.ListByProjectFn(ctx, projectID, params)
	}
	return nil, 0, nil
}

// ─── CronJobRunStore ─────────────────────────────────────────────

type FakeCronJobRunStore struct {
	GetByIDFn       func(ctx context.Context, id uuid.UUID) (*model.CronJobRun, error)
	CreateFn        func(ctx context.Context, run *model.CronJobRun) error
	UpdateFn        func(ctx context.Context, run *model.CronJobRun) error
	ListByCronJobFn func(ctx context.Context, cronJobID uuid.UUID, params store.ListParams) ([]model.CronJobRun, int, error)
}

func (f *FakeCronJobRunStore) GetByID(ctx context.Context, id uuid.UUID) (*model.CronJobRun, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeCronJobRunStore) Create(ctx context.Context, run *model.CronJobRun) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, run)
	}
	return nil
}

func (f *FakeCronJobRunStore) Update(ctx context.Context, run *model.CronJobRun) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, run)
	}
	return nil
}

func (f *FakeCronJobRunStore) ListByCronJob(ctx context.Context, cronJobID uuid.UUID, params store.ListParams) ([]model.CronJobRun, int, error) {
	if f.ListByCronJobFn != nil {
		return f.ListByCronJobFn(ctx, cronJobID, params)
	}
	return nil, 0, nil
}

// ─── DatabaseBackupStore ─────────────────────────────────────────

type FakeDatabaseBackupStore struct {
	CreateFn         func(ctx context.Context, backup *model.DatabaseBackup) error
	UpdateFn         func(ctx context.Context, backup *model.DatabaseBackup) error
	GetByIDFn        func(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error)
	ListByDatabaseFn func(ctx context.Context, databaseID uuid.UUID, params store.ListParams) ([]model.DatabaseBackup, int, error)
}

func (f *FakeDatabaseBackupStore) Create(ctx context.Context, backup *model.DatabaseBackup) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, backup)
	}
	return nil
}

func (f *FakeDatabaseBackupStore) Update(ctx context.Context, backup *model.DatabaseBackup) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, backup)
	}
	return nil
}

func (f *FakeDatabaseBackupStore) GetByID(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeDatabaseBackupStore) ListByDatabase(ctx context.Context, databaseID uuid.UUID, params store.ListParams) ([]model.DatabaseBackup, int, error) {
	if f.ListByDatabaseFn != nil {
		return f.ListByDatabaseFn(ctx, databaseID, params)
	}
	return nil, 0, nil
}

// ─── TeamStore ───────────────────────────────────────────────────

type FakeTeamStore struct {
	CreateFn        func(ctx context.Context, team *model.Team) error
	UpdateFn        func(ctx context.Context, team *model.Team) error
	DeleteFn        func(ctx context.Context, id uuid.UUID) error
	GetByIDFn       func(ctx context.Context, id uuid.UUID) (*model.Team, error)
	ListByOrgFn     func(ctx context.Context, orgID uuid.UUID) ([]model.Team, error)
	CountProjectsFn func(ctx context.Context, teamID uuid.UUID) (int, error)
}

func (f *FakeTeamStore) Create(ctx context.Context, team *model.Team) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, team)
	}
	return nil
}

func (f *FakeTeamStore) Update(ctx context.Context, team *model.Team) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, team)
	}
	return nil
}

func (f *FakeTeamStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeTeamStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Team, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *FakeTeamStore) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.Team, error) {
	if f.ListByOrgFn != nil {
		return f.ListByOrgFn(ctx, orgID)
	}
	return nil, nil
}

func (f *FakeTeamStore) CountProjects(ctx context.Context, teamID uuid.UUID) (int, error) {
	if f.CountProjectsFn != nil {
		return f.CountProjectsFn(ctx, teamID)
	}
	return 0, nil
}

// ─── TeamMemberStore ─────────────────────────────────────────────

type FakeTeamMemberStore struct {
	AddFn               func(ctx context.Context, teamID, userID uuid.UUID) error
	RemoveFn            func(ctx context.Context, teamID, userID uuid.UUID) error
	ListByTeamFn        func(ctx context.Context, teamID uuid.UUID) ([]model.TeamMember, error)
	ListByUserFn        func(ctx context.Context, userID uuid.UUID) ([]model.TeamMember, error)
	ListTeamIDsByUserFn func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	ListUsersByTeamFn   func(ctx context.Context, teamID uuid.UUID) ([]model.OrgMember, error)
}

func (f *FakeTeamMemberStore) Add(ctx context.Context, teamID, userID uuid.UUID) error {
	if f.AddFn != nil {
		return f.AddFn(ctx, teamID, userID)
	}
	return nil
}

func (f *FakeTeamMemberStore) Remove(ctx context.Context, teamID, userID uuid.UUID) error {
	if f.RemoveFn != nil {
		return f.RemoveFn(ctx, teamID, userID)
	}
	return nil
}

func (f *FakeTeamMemberStore) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]model.TeamMember, error) {
	if f.ListByTeamFn != nil {
		return f.ListByTeamFn(ctx, teamID)
	}
	return nil, nil
}

func (f *FakeTeamMemberStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.TeamMember, error) {
	if f.ListByUserFn != nil {
		return f.ListByUserFn(ctx, userID)
	}
	return nil, nil
}

func (f *FakeTeamMemberStore) ListTeamIDsByUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	if f.ListTeamIDsByUserFn != nil {
		return f.ListTeamIDsByUserFn(ctx, userID)
	}
	return nil, nil
}

func (f *FakeTeamMemberStore) ListUsersByTeam(ctx context.Context, teamID uuid.UUID) ([]model.OrgMember, error) {
	if f.ListUsersByTeamFn != nil {
		return f.ListUsersByTeamFn(ctx, teamID)
	}
	return nil, nil
}

// ─── InvitationStore ─────────────────────────────────────────────

type FakeInvitationStore struct {
	CreateFn     func(ctx context.Context, inv *model.Invitation) error
	GetByIDFn    func(ctx context.Context, id uuid.UUID) (*model.Invitation, error)
	GetByTokenFn func(ctx context.Context, token string) (*model.Invitation, error)
	ListByOrgFn  func(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error)
	DeleteFn     func(ctx context.Context, id uuid.UUID) error
	UpdateFn     func(ctx context.Context, inv *model.Invitation) error
}

func (f *FakeInvitationStore) Create(ctx context.Context, inv *model.Invitation) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, inv)
	}
	return nil
}

func (f *FakeInvitationStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (f *FakeInvitationStore) GetByToken(ctx context.Context, token string) (*model.Invitation, error) {
	if f.GetByTokenFn != nil {
		return f.GetByTokenFn(ctx, token)
	}
	return nil, nil
}

func (f *FakeInvitationStore) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error) {
	if f.ListByOrgFn != nil {
		return f.ListByOrgFn(ctx, orgID)
	}
	return nil, nil
}

func (f *FakeInvitationStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.DeleteFn != nil {
		return f.DeleteFn(ctx, id)
	}
	return nil
}

func (f *FakeInvitationStore) Update(ctx context.Context, inv *model.Invitation) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, inv)
	}
	return nil
}

// ─── NotificationChannelStore ────────────────────────────────────

type FakeNotificationChannelStore struct {
	GetByOrgAndTypeFn func(ctx context.Context, orgID uuid.UUID, channelType string) (*model.NotificationChannel, error)
	ListByOrgFn       func(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error)
	ListAllEnabledFn  func(ctx context.Context) ([]model.NotificationChannel, error)
	UpsertFn          func(ctx context.Context, channel *model.NotificationChannel) error
}

func (f *FakeNotificationChannelStore) GetByOrgAndType(ctx context.Context, orgID uuid.UUID, channelType string) (*model.NotificationChannel, error) {
	if f.GetByOrgAndTypeFn != nil {
		return f.GetByOrgAndTypeFn(ctx, orgID, channelType)
	}
	return nil, nil
}

func (f *FakeNotificationChannelStore) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
	if f.ListByOrgFn != nil {
		return f.ListByOrgFn(ctx, orgID)
	}
	return nil, nil
}

func (f *FakeNotificationChannelStore) ListAllEnabled(ctx context.Context) ([]model.NotificationChannel, error) {
	if f.ListAllEnabledFn != nil {
		return f.ListAllEnabledFn(ctx)
	}
	return nil, nil
}

func (f *FakeNotificationChannelStore) Upsert(ctx context.Context, channel *model.NotificationChannel) error {
	if f.UpsertFn != nil {
		return f.UpsertFn(ctx, channel)
	}
	return nil
}

// ─── SystemBackupStore ───────────────────────────────────────────

type FakeSystemBackupStore struct {
	CreateFn  func(ctx context.Context, backup *model.SystemBackup) error
	UpdateFn  func(ctx context.Context, backup *model.SystemBackup) error
	ListFn    func(ctx context.Context, params store.ListParams) ([]model.SystemBackup, int, error)
	GetByIDFn func(ctx context.Context, id uuid.UUID) (*model.SystemBackup, error)
}

func (f *FakeSystemBackupStore) Create(ctx context.Context, backup *model.SystemBackup) error {
	if f.CreateFn != nil {
		return f.CreateFn(ctx, backup)
	}
	return nil
}

func (f *FakeSystemBackupStore) Update(ctx context.Context, backup *model.SystemBackup) error {
	if f.UpdateFn != nil {
		return f.UpdateFn(ctx, backup)
	}
	return nil
}

func (f *FakeSystemBackupStore) List(ctx context.Context, params store.ListParams) ([]model.SystemBackup, int, error) {
	if f.ListFn != nil {
		return f.ListFn(ctx, params)
	}
	return nil, 0, nil
}

func (f *FakeSystemBackupStore) GetByID(ctx context.Context, id uuid.UUID) (*model.SystemBackup, error) {
	if f.GetByIDFn != nil {
		return f.GetByIDFn(ctx, id)
	}
	return nil, nil
}

// ─── MetricsStore ────────────────────────────────────────────────

type FakeMetricsStore struct {
	SnapshotsStore *FakeMetricSnapshotStore
	EventsStore    *FakeMetricEventStore
	AlertsStore    *FakeMetricAlertStore
}

func NewFakeMetricsStore() *FakeMetricsStore {
	return &FakeMetricsStore{
		SnapshotsStore: &FakeMetricSnapshotStore{},
		EventsStore:    &FakeMetricEventStore{},
		AlertsStore:    &FakeMetricAlertStore{},
	}
}

var _ store.MetricsStore = (*FakeMetricsStore)(nil)

func (f *FakeMetricsStore) Snapshots() store.MetricSnapshotStore { return f.SnapshotsStore }
func (f *FakeMetricsStore) Events() store.MetricEventStore       { return f.EventsStore }
func (f *FakeMetricsStore) Alerts() store.MetricAlertStore       { return f.AlertsStore }

type FakeMetricSnapshotStore struct {
	InsertBatchFn     func(ctx context.Context, snapshots []model.MetricSnapshot) error
	QueryFn           func(ctx context.Context, q store.SnapshotQuery) ([]model.MetricSnapshot, error)
	DeleteOlderThanFn func(ctx context.Context, before time.Time) (int64, error)
}

func (f *FakeMetricSnapshotStore) InsertBatch(ctx context.Context, snapshots []model.MetricSnapshot) error {
	if f.InsertBatchFn != nil {
		return f.InsertBatchFn(ctx, snapshots)
	}
	return nil
}

func (f *FakeMetricSnapshotStore) Query(ctx context.Context, q store.SnapshotQuery) ([]model.MetricSnapshot, error) {
	if f.QueryFn != nil {
		return f.QueryFn(ctx, q)
	}
	return nil, nil
}

func (f *FakeMetricSnapshotStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if f.DeleteOlderThanFn != nil {
		return f.DeleteOlderThanFn(ctx, before)
	}
	return 0, nil
}

type FakeMetricEventStore struct {
	UpsertBatchFn     func(ctx context.Context, events []model.MetricEvent) error
	ListFn            func(ctx context.Context, q store.EventQuery) ([]model.MetricEvent, int, error)
	DeleteOlderThanFn func(ctx context.Context, before time.Time) (int64, error)
}

func (f *FakeMetricEventStore) UpsertBatch(ctx context.Context, events []model.MetricEvent) error {
	if f.UpsertBatchFn != nil {
		return f.UpsertBatchFn(ctx, events)
	}
	return nil
}

func (f *FakeMetricEventStore) List(ctx context.Context, q store.EventQuery) ([]model.MetricEvent, int, error) {
	if f.ListFn != nil {
		return f.ListFn(ctx, q)
	}
	return nil, 0, nil
}

func (f *FakeMetricEventStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if f.DeleteOlderThanFn != nil {
		return f.DeleteOlderThanFn(ctx, before)
	}
	return 0, nil
}

type FakeMetricAlertStore struct {
	InsertFn                   func(ctx context.Context, alert *model.MetricAlert) error
	GetActiveByRuleAndSourceFn func(ctx context.Context, ruleName, sourceName string) (*model.MetricAlert, error)
	ResolveFn                  func(ctx context.Context, id uuid.UUID) error
	MarkNotifiedFn             func(ctx context.Context, id uuid.UUID) error
	ListActiveFn               func(ctx context.Context) ([]model.MetricAlert, error)
	ListFn                     func(ctx context.Context, q store.AlertQuery) ([]model.MetricAlert, int, error)
	DeleteOlderThanFn          func(ctx context.Context, before time.Time) (int64, error)
}

func (f *FakeMetricAlertStore) Insert(ctx context.Context, alert *model.MetricAlert) error {
	if f.InsertFn != nil {
		return f.InsertFn(ctx, alert)
	}
	return nil
}

func (f *FakeMetricAlertStore) GetActiveByRuleAndSource(ctx context.Context, ruleName, sourceName string) (*model.MetricAlert, error) {
	if f.GetActiveByRuleAndSourceFn != nil {
		return f.GetActiveByRuleAndSourceFn(ctx, ruleName, sourceName)
	}
	return nil, nil
}

func (f *FakeMetricAlertStore) Resolve(ctx context.Context, id uuid.UUID) error {
	if f.ResolveFn != nil {
		return f.ResolveFn(ctx, id)
	}
	return nil
}

func (f *FakeMetricAlertStore) MarkNotified(ctx context.Context, id uuid.UUID) error {
	if f.MarkNotifiedFn != nil {
		return f.MarkNotifiedFn(ctx, id)
	}
	return nil
}

func (f *FakeMetricAlertStore) ListActive(ctx context.Context) ([]model.MetricAlert, error) {
	if f.ListActiveFn != nil {
		return f.ListActiveFn(ctx)
	}
	return nil, nil
}

func (f *FakeMetricAlertStore) List(ctx context.Context, q store.AlertQuery) ([]model.MetricAlert, int, error) {
	if f.ListFn != nil {
		return f.ListFn(ctx, q)
	}
	return nil, 0, nil
}

func (f *FakeMetricAlertStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if f.DeleteOlderThanFn != nil {
		return f.DeleteOlderThanFn(ctx, before)
	}
	return 0, nil
}
