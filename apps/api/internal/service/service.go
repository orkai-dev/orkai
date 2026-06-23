package service

import (
	"log/slog"

	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	pagesaws "github.com/orkai-dev/orkai/apps/api/internal/pages/aws"
	pagescf "github.com/orkai-dev/orkai/apps/api/internal/pages/cloudflare"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Container holds all services with their dependencies.
type Container struct {
	Auth         *AuthService
	Project      *ProjectService
	App          *AppService
	Deploy       *DeployService
	Build        *BuildService
	Database     *DatabaseService
	Template     *TemplateService
	Domain       *DomainService
	Setting      *SettingService
	Node         *NodeService
	Resource     *ResourceService
	Metrics      *MetricsCollector
	CronJob      *CronJobService
	Team         *TeamService
	Authz        *Authz
	Notification *NotificationService
	Version      *VersionService
	SystemBackup *SystemBackupService
	RegistryAuth *RegistryAuth
	Page         *PageService
	PageDeploy   *PageDeployService
	Worker       *WorkerService
	WorkerDeploy *WorkerDeployService
	APIKey       *APIKeyService
	Targets      *orchestrator.TargetRegistry
	Providers    *providers.Registry
}

// NewContainer creates all services with shared dependencies.
func NewContainer(
	s store.Store,
	metricsStore store.MetricsStore,
	targets *orchestrator.TargetRegistry,
	jwtManager *auth.JWTManager,
	logger *slog.Logger,
	dbURL string,
	setupSecret string,
	queue Enqueuer,
) *Container {
	prov := providers.New(s.Settings(), logger)
	settingSvc := NewSettingService(s, targets, logger)
	notifSvc := NewNotificationService(s, settingSvc, logger)
	domainSvc := NewDomainService(s, targets, logger, settingSvc, notifSvc)
	buildSvc := NewBuildService(s, targets, prov, logger)
	registryAuth := NewRegistryAuth(s, targets, prov, logger)

	pagesRegistry := pages.NewRegistry(pagesaws.New(), pagescf.New())
	pagePublishSvc := NewPagePublishService(s, prov, logger)

	return &Container{
		Auth:         NewAuthService(s, jwtManager, logger),
		Project:      NewProjectService(s, targets, logger, notifSvc),
		App:          NewAppService(s, targets, logger, domainSvc, registryAuth, notifSvc),
		Deploy:       NewDeployService(s, targets, logger, buildSvc, notifSvc, registryAuth, queue),
		Build:        buildSvc,
		Database:     NewDatabaseService(s, targets, logger, queue, prov, notifSvc),
		Template:     NewTemplateService(s, logger),
		Domain:       domainSvc,
		Setting:      settingSvc,
		Node:         NewNodeService(s, targets, logger, notifSvc),
		Resource:     NewResourceService(s, prov, logger, notifSvc),
		Metrics:      NewMetricsCollector(metricsStore, s, targets, logger, notifSvc),
		CronJob:      NewCronJobService(s, targets, logger, notifSvc),
		Team:         NewTeamService(s, jwtManager, logger, notifSvc),
		Authz:        NewAuthz(s),
		Notification: notifSvc,
		Version:      NewVersionService(s, logger),
		SystemBackup: NewSystemBackupService(s, settingSvc, dbURL, logger, queue, prov),
		RegistryAuth: registryAuth,
		Page:         NewPageService(s, pagesRegistry, logger, notifSvc),
		PageDeploy:   NewPageDeployService(s, pagesRegistry, pagePublishSvc, notifSvc, queue, targets, logger),
		Worker:       NewWorkerService(s, targets, prov, logger, notifSvc),
		WorkerDeploy: NewWorkerDeployService(s, notifSvc, queue, targets, prov, logger),
		APIKey:       NewAPIKeyService(s, logger, notifSvc),
		Targets:      targets,
		Providers:    prov,
	}
}
