package server

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	v1 "github.com/orkai-dev/orkai/apps/api/internal/api/v1"
	"github.com/orkai-dev/orkai/apps/api/internal/api/ws"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// RouterDeps holds dependencies required by the router.
type RouterDeps struct {
	Services    *service.Container
	JWTManager  *auth.JWTManager
	Targets     *orchestrator.TargetRegistry
	Store       store.Store
	SSEBroker   *ws.SSEBroker
	AppURL      string // Public URL of the Orkai instance
	SetupSecret string // Secret for unauthenticated setup operations
	Logger      *slog.Logger
}

// NewRouter creates and configures the Gin engine with all routes.
func NewRouter(deps *RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Global middleware
	r.Use(
		middleware.Recovery(deps.Logger),
		middleware.Sentry(),
		middleware.Branding(),
		middleware.RequestID(),
		middleware.Logger(deps.Logger),
		middleware.CORS(),
	)

	// Health check
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// WebSocket / SSE routes (auth via query param token)
	sessionValidator := middleware.NewSessionValidator(deps.JWTManager, deps.Store.Users(), deps.Services.APIKey)
	wsGroup := r.Group("/ws")
	wsGroup.Use(sessionValidator.WSAuth())
	{
		logsHandler := ws.NewLogsHandler(deps.Store, deps.Targets, deps.Logger)
		wsGroup.GET("/logs/:appId", logsHandler.Handle)

		wsGroup.GET("/events", deps.SSEBroker.ServeHTTP)

		nodeLogsHandler := ws.NewNodeLogsHandler(deps.Services.Node, deps.Logger)
		wsGroup.GET("/nodes/:id/logs", nodeLogsHandler.Handle)

		terminalHandler := ws.NewTerminalHandler(deps.Store, deps.Targets, deps.Logger)
		wsGroup.GET("/terminal/:appId", terminalHandler.Handle)
	}

	// API v1
	apiV1 := r.Group("/api/v1")
	{
		// Public routes
		authHandler := v1.NewAuthHandler(deps.Services.Auth, sessionValidator)
		loginRL := middleware.RateLimit(10, 1*time.Minute)
		apiV1.GET("/auth/setup-status", authHandler.SetupStatus)
		apiV1.POST("/auth/register", loginRL, authHandler.Register)
		apiV1.POST("/auth/login", loginRL, authHandler.Login)
		apiV1.POST("/auth/refresh", authHandler.Refresh)

		oauthLogin := v1.NewOAuthLoginHandler(deps.Store, deps.Services.Auth, deps.JWTManager, deps.AppURL, deps.Logger)
		apiV1.GET("/auth/providers", oauthLogin.Providers)
		apiV1.GET("/auth/oauth/:provider/login", loginRL, oauthLogin.Login)
		apiV1.GET("/auth/oauth/:provider/callback", oauthLogin.Callback)
		apiV1.POST("/auth/oauth/2fa", loginRL, oauthLogin.Complete2FA)

		// GitHub OAuth/Manifest (public - GitHub redirects here without JWT)
		githubOAuth := v1.NewGitHubOAuthHandler(deps.Store, deps.Services.Resource, deps.AppURL, deps.Logger)
		apiV1.GET("/auth/github/callback", githubOAuth.Callback)
		apiV1.GET("/auth/github/setup/callback", githubOAuth.SetupCallback)

		// Webhooks (public - called by GitHub/GitLab without JWT)
		webhookHandler := v1.NewWebhookHandler(deps.Store, deps.Services.Deploy, deps.Services.Providers, deps.Logger)
		apiV1.POST("/webhooks/github/:appId", webhookHandler.GitHub)
		apiV1.POST("/webhooks/gitlab/:appId", webhookHandler.GitLab)

		// Team invitations (public — invitee may not have an account yet)
		teamPublic := v1.NewTeamHandler(deps.Services.Team, deps.Services.Notification, deps.AppURL, sessionValidator)
		apiV1.GET("/team/invitations/info", teamPublic.GetInvitationByToken)
		apiV1.POST("/team/invitations/accept-public", loginRL, teamPublic.AcceptInvitationPublic)

		// System restore (requires setup secret + only works on uninitialized system)
		restoreRL := middleware.RateLimit(5, 5*time.Minute)
		setupAuth := middleware.RequireSetupSecret(deps.SetupSecret)
		sysBackupPublic := v1.NewSystemBackupHandler(deps.Services.SystemBackup)
		apiV1.POST("/system/restore/scan", restoreRL, setupAuth, sysBackupPublic.ScanS3Backups)
		apiV1.POST("/system/restore/execute", restoreRL, setupAuth, sysBackupPublic.RestoreFromS3)

		// Protected routes
		protected := apiV1.Group("")
		protected.Use(sessionValidator.Auth())
		{
			// Project-scoped access guards (admin = org-wide, member = own teams)
			guards := v1.NewGuards(deps.Store, deps.Services.Authz)

			// adminOnly gates infrastructure + org-management endpoints to admins.
			adminOnly := protected.Group("", middleware.RequireRole("admin"))

			// Auth
			protected.GET("/auth/me", authHandler.Me)
			protected.POST("/auth/logout", authHandler.Logout)
			protected.PATCH("/auth/profile", authHandler.UpdateProfile)
			protected.POST("/auth/change-password", authHandler.ChangePassword)
			protected.GET("/auth/avatars", authHandler.ListAvatars)
			protected.POST("/auth/2fa/setup", authHandler.Setup2FA)
			protected.POST("/auth/2fa/verify", authHandler.Verify2FA)
			protected.POST("/auth/2fa/disable", authHandler.Disable2FA)

			// API keys (personal access tokens)
			apiKeys := v1.NewAPIKeyHandler(deps.Services.APIKey)
			protected.GET("/api-keys", apiKeys.List)
			protected.POST("/api-keys", apiKeys.Create)
			protected.DELETE("/api-keys/:id", apiKeys.Revoke)

			// GitHub OAuth
			protected.GET("/auth/github/connect", githubOAuth.Connect)
			protected.GET("/auth/github/setup", githubOAuth.SetupManifest)
			protected.GET("/auth/github/status", githubOAuth.GitHubStatus)

			// Projects
			projects := v1.NewProjectHandler(deps.Services.Project)
			protected.GET("/projects", projects.List)
			protected.POST("/projects", middleware.RequireRole("admin"), projects.Create)

			projectGroup := protected.Group("/projects/:id")
			projectGroup.Use(guards.Project("id"))
			{
				projectGroup.GET("", projects.Get)
				projectGroup.PATCH("", projects.Update)
				projectGroup.DELETE("", projects.Delete)
				projectGroup.PUT("/env", projects.UpdateEnv)

				appHandler := v1.NewAppHandler(deps.Services.App, deps.Services.Metrics, deps.Store, deps.Services.Authz)
				projectGroup.GET("/apps", appHandler.ListByProject)

				dbHandler := v1.NewDatabaseHandler(deps.Services.Database, deps.Store, deps.Services.Authz)
				projectGroup.GET("/databases", dbHandler.ListByProject)

				cronJobHandler := v1.NewCronJobHandler(deps.Services.CronJob, deps.Services.Authz)
				projectGroup.GET("/cronjobs", cronJobHandler.ListByProject)

				pageHandler := v1.NewPageHandler(deps.Services.Page, deps.Services.PageDeploy, deps.Store, deps.Services.Authz)
				projectGroup.GET("/pages", pageHandler.ListByProject)

				workerHandler := v1.NewWorkerHandler(deps.Services.Worker, deps.Services.WorkerDeploy, deps.Store, deps.Services.Authz)
				projectGroup.GET("/workers", workerHandler.ListByProject)
			}

			// Applications (flat)
			appHandler := v1.NewAppHandler(deps.Services.App, deps.Services.Metrics, deps.Store, deps.Services.Authz)
			protected.GET("/apps", appHandler.ListAll)
			protected.POST("/apps", appHandler.Create)

			// All /apps/:id routes require project access verification
			appByID := protected.Group("/apps/:id")
			appByID.Use(guards.App("id"))
			{
				appByID.GET("", appHandler.Get)
				appByID.DELETE("", appHandler.Delete)
				appByID.POST("/scale", appHandler.Scale)
				appByID.PUT("/env", appHandler.UpdateEnv)
				appByID.GET("/status", appHandler.GetStatus)
				appByID.GET("/capabilities", appHandler.GetCapabilities)
				appByID.GET("/pods", appHandler.GetPods)
				appByID.GET("/metrics", appHandler.GetMetrics)
				appByID.POST("/restart", appHandler.Restart)
				appByID.POST("/stop", appHandler.Stop)
				appByID.POST("/clear-cache", appHandler.ClearBuildCache)
				appByID.PATCH("", appHandler.Update)
				appByID.GET("/pods/:podName/events", appHandler.GetPodEvents)
				appByID.GET("/webhook", appHandler.GetWebhookConfig)
				appByID.POST("/webhook/enable", appHandler.EnableWebhook)
				appByID.POST("/webhook/disable", appHandler.DisableWebhook)
				appByID.POST("/webhook/regenerate", appHandler.RegenerateWebhook)
				appByID.GET("/secrets", appHandler.GetSecrets)
				appByID.PUT("/secrets", appHandler.UpdateSecrets)
			}
			// Deployments under apps
			deploys := v1.NewDeployHandler(deps.Services.Deploy, deps.Store, deps.Services.Authz)
			protected.POST("/apps/:id/deploy", guards.App("id"), deploys.Trigger)
			protected.GET("/apps/:id/deployments", guards.App("id"), deploys.List)

			// Domains under apps
			domains := v1.NewDomainHandler(deps.Services.Domain, deps.Store)
			protected.GET("/apps/:id/domains", guards.App("id"), domains.ListByApp)
			protected.POST("/apps/:id/domains", guards.App("id"), domains.Create)
			protected.POST("/apps/:id/domains/generate", guards.App("id"), domains.Generate)

			// Deployments (flat)
			protected.GET("/deployments", deploys.ListAll)
			protected.GET("/deployments/queue", deploys.ListQueue)
			protected.GET("/deployments/:id", guards.Deployment("id"), deploys.Get)
			protected.POST("/deployments/:id/cancel", guards.Deployment("id"), deploys.Cancel)
			protected.POST("/deployments/:id/rollback", guards.Deployment("id"), deploys.Rollback)

			// CronJobs (flat)
			cronJobs := v1.NewCronJobHandler(deps.Services.CronJob, deps.Services.Authz)
			protected.POST("/cronjobs", cronJobs.Create)
			protected.GET("/cronjobs/:id", guards.CronJob("id"), cronJobs.Get)
			protected.PATCH("/cronjobs/:id", guards.CronJob("id"), cronJobs.Update)
			protected.DELETE("/cronjobs/:id", guards.CronJob("id"), cronJobs.Delete)
			protected.POST("/cronjobs/:id/trigger", guards.CronJob("id"), cronJobs.Trigger)
			protected.GET("/cronjobs/:id/runs", guards.CronJob("id"), cronJobs.ListRuns)

			// Domains (flat - for delete and update)
			protected.DELETE("/domains/:id", guards.Domain("id"), domains.Delete)
			protected.PATCH("/domains/:id", guards.Domain("id"), domains.Update)

			// Databases (flat)
			dbHandler := v1.NewDatabaseHandler(deps.Services.Database, deps.Store, deps.Services.Authz)
			protected.GET("/databases", dbHandler.ListAll)
			protected.GET("/databases/versions", dbHandler.ListVersions)
			protected.GET("/databases/used-ports", dbHandler.UsedPorts)
			protected.POST("/databases", dbHandler.Create)
			protected.GET("/databases/:id", guards.Database("id"), dbHandler.Get)
			protected.DELETE("/databases/:id", guards.Database("id"), dbHandler.Delete)
			protected.GET("/databases/:id/credentials", guards.Database("id"), dbHandler.GetCredentials)
			protected.GET("/databases/:id/status", guards.Database("id"), dbHandler.GetStatus)
			protected.GET("/databases/:id/pods", guards.Database("id"), dbHandler.GetPods)
			protected.POST("/databases/:id/backups", guards.Database("id"), dbHandler.TriggerBackup)
			protected.GET("/databases/:id/backups", guards.Database("id"), dbHandler.ListBackups)
			protected.POST("/databases/:id/backups/:backupId/restore", guards.Database("id"), dbHandler.RestoreBackup)
			protected.POST("/databases/:id/external-access", guards.Database("id"), dbHandler.UpdateExternalAccess)
			protected.PUT("/databases/:id/backup-config", guards.Database("id"), dbHandler.UpdateBackupConfig)

			// Pages (flat)
			pageHandler := v1.NewPageHandler(deps.Services.Page, deps.Services.PageDeploy, deps.Store, deps.Services.Authz)
			protected.GET("/pages", pageHandler.ListAll)
			protected.POST("/pages", pageHandler.Create)
			protected.GET("/pages/:id", guards.Page("id"), pageHandler.Get)
			protected.PATCH("/pages/:id", guards.Page("id"), pageHandler.Update)
			protected.DELETE("/pages/:id", guards.Page("id"), pageHandler.Delete)
			protected.POST("/pages/:id/deploy", guards.Page("id"), pageHandler.Deploy)
			protected.GET("/pages/:id/deployments", guards.Page("id"), pageHandler.ListDeployments)
			protected.GET("/pages/:id/deployments/:deployId", guards.Page("id"), pageHandler.GetDeployment)

			// Workers (flat)
			workerHandler := v1.NewWorkerHandler(deps.Services.Worker, deps.Services.WorkerDeploy, deps.Store, deps.Services.Authz)
			protected.GET("/workers", workerHandler.ListAll)
			protected.POST("/workers", workerHandler.Create)
			protected.GET("/workers/:id", guards.Worker("id"), workerHandler.Get)
			protected.PATCH("/workers/:id", guards.Worker("id"), workerHandler.Update)
			protected.DELETE("/workers/:id", guards.Worker("id"), workerHandler.Delete)
			protected.POST("/workers/:id/deploy", guards.Worker("id"), workerHandler.Deploy)
			protected.POST("/workers/:id/confirm-r2", guards.Worker("id"), workerHandler.ConfirmR2)
			protected.GET("/workers/:id/deployments", guards.Worker("id"), workerHandler.ListDeployments)
			protected.GET("/workers/:id/deployments/:deployId", guards.Worker("id"), workerHandler.GetDeployment)

			templates := v1.NewTemplateHandler(deps.Services.Template)
			protected.GET("/templates", templates.List)
			protected.GET("/templates/:id", templates.Get)

			// Cluster (admin only)
			cluster := v1.NewClusterHandler(deps.Targets, deps.Store, deps.Services.Notification)
			adminOnly.GET("/cluster/nodes", cluster.GetNodes)
			adminOnly.GET("/cluster/capabilities", cluster.GetCapabilities)
			adminOnly.GET("/cluster/metrics", cluster.GetMetrics)
			adminOnly.GET("/cluster/pods", cluster.GetAllPods)
			adminOnly.GET("/cluster/events", cluster.GetEvents)
			adminOnly.GET("/cluster/pvcs", cluster.GetPVCs)
			adminOnly.GET("/cluster/storage-classes", cluster.GetStorageClasses)
			adminOnly.GET("/cluster/namespaces", cluster.GetNamespaces)
			adminOnly.GET("/cluster/node-metrics", cluster.GetNodeMetrics)
			adminOnly.GET("/cluster/topology", cluster.GetTopology)
			adminOnly.GET("/cluster/node-pools", cluster.GetNodePools)
			adminOnly.PUT("/cluster/nodes/:name/pool", cluster.SetNodePool)
			adminOnly.GET("/cluster/traefik-config", cluster.GetTraefikConfig)
			adminOnly.PUT("/cluster/traefik-config", cluster.UpdateTraefikConfig)
			adminOnly.POST("/cluster/traefik-restart", cluster.RestartTraefik)
			adminOnly.GET("/cluster/traefik-status", cluster.GetTraefikStatus)
			adminOnly.GET("/cluster/helm-releases", cluster.GetHelmReleases)
			adminOnly.GET("/cluster/daemonsets", cluster.GetDaemonSets)
			adminOnly.DELETE("/cluster/pvcs/:namespace/:name", cluster.DeletePVC)
			adminOnly.PUT("/cluster/pvcs/:namespace/:name/expand", cluster.ExpandPVC)
			adminOnly.GET("/cluster/cleanup/stats", cluster.GetCleanupStats)
			adminOnly.POST("/cluster/cleanup/evicted-pods", cluster.CleanupEvictedPods)
			adminOnly.POST("/cluster/cleanup/failed-pods", cluster.CleanupFailedPods)
			adminOnly.POST("/cluster/cleanup/completed-pods", cluster.CleanupCompletedPods)
			adminOnly.POST("/cluster/cleanup/stale-replicasets", cluster.CleanupStaleReplicaSets)
			adminOnly.POST("/cluster/cleanup/completed-jobs", cluster.CleanupCompletedJobs)
			adminOnly.POST("/cluster/cleanup/orphan-ingresses", cluster.CleanupOrphanIngresses)

			// Monitoring
			monitoring := v1.NewMonitoringHandler(deps.Services.Metrics)
			protected.GET("/monitoring/snapshots", monitoring.GetSnapshots)
			protected.GET("/monitoring/events", monitoring.GetEvents)
			protected.GET("/monitoring/alerts", monitoring.GetAlerts)
			protected.GET("/monitoring/alerts/active", monitoring.GetActiveAlerts)
			protected.POST("/monitoring/alerts/:id/resolve", monitoring.ResolveAlert)

			// Settings (admin only)
			settings := v1.NewSettingHandler(deps.Services.Setting)
			adminOnly.GET("/settings", settings.GetAll)
			adminOnly.PUT("/settings", settings.Update)
			adminOnly.GET("/settings/verify-domain", settings.VerifyDomain)

			// Notifications (admin only)
			notif := v1.NewNotificationHandler(deps.Services.Notification)
			adminOnly.GET("/notifications/channels", notif.ListChannels)
			adminOnly.GET("/notifications/events", notif.ListEvents)
			adminOnly.PUT("/notifications/channels", notif.SaveChannel)
			adminOnly.POST("/notifications/test", notif.TestChannel)
			adminOnly.GET("/settings/smtp", notif.GetSMTPConfig)
			adminOnly.PUT("/settings/smtp", notif.SaveSMTPConfig)
			adminOnly.POST("/settings/smtp/test", notif.TestSMTP)

			// Nodes
			nodeHandler := v1.NewNodeHandler(deps.Services.Node)
			protected.GET("/nodes", nodeHandler.List)
			protected.POST("/nodes", nodeHandler.Create)
			protected.POST("/nodes/:id/initialize", nodeHandler.Initialize)
			protected.DELETE("/nodes/:id", nodeHandler.Delete)

			// Team management
			team := v1.NewTeamHandler(deps.Services.Team, deps.Services.Notification, deps.AppURL, sessionValidator)
			protected.GET("/team/members", team.ListMembers)
			protected.PATCH("/team/members/:id/role", middleware.RequireRole("admin"), team.UpdateMemberRole)
			protected.DELETE("/team/members/:id", middleware.RequireRole("admin"), team.RemoveMember)
			protected.POST("/team/invitations", middleware.RequireRole("admin"), team.InviteMember)
			protected.GET("/team/invitations", middleware.RequireRole("admin"), team.ListInvitations)
			protected.DELETE("/team/invitations/:id", middleware.RequireRole("admin"), team.CancelInvitation)
			protected.POST("/team/invitations/accept", team.AcceptInvitation)

			// Teams (admins manage teams + membership)
			protected.GET("/teams", team.ListTeams)
			protected.POST("/teams", middleware.RequireRole("admin"), team.CreateTeam)
			protected.DELETE("/teams/:id", middleware.RequireRole("admin"), team.DeleteTeam)
			protected.GET("/teams/:id/members", middleware.RequireRole("admin"), team.ListTeamMembers)
			protected.POST("/teams/:id/members", middleware.RequireRole("admin"), team.AddTeamMember)
			protected.DELETE("/teams/:id/members/:userId", middleware.RequireRole("admin"), team.RemoveTeamMember)

			// Version & Upgrade
			// getVersion godoc
			// @Summary      Get version info
			// @Tags         system
			// @Produce      json
			// @Success      200 {object} apidocs.VersionInfo
			// @Failure      401 {object} apidocs.ProblemDetail
			// @Security     BearerAuth
			// @Router       /version [get]
			protected.GET("/version", func(c *gin.Context) {
				c.JSON(200, deps.Services.Version.GetVersionInfo(c.Request.Context()))
			})
			// triggerUpgrade godoc
			// @Summary      Trigger system upgrade
			// @Description  Requires admin role.
			// @Tags         system
			// @Produce      json
			// @Success      200 {object} apidocs.MessageResponse
			// @Failure      400 {object} apidocs.ProblemDetail
			// @Failure      401 {object} apidocs.ProblemDetail
			// @Failure      403 {object} apidocs.ProblemDetail
			// @Security     BearerAuth
			// @Router       /system/upgrade [post]
			protected.POST("/system/upgrade", middleware.RequireRole("admin"), func(c *gin.Context) {
				if err := deps.Services.Version.TriggerUpgrade(); err != nil {
					c.JSON(400, gin.H{"detail": err.Error()})
					return
				}
				c.JSON(200, gin.H{"message": "upgrade started"})
			})
			// getUpgradeStatus godoc
			// @Summary      Get upgrade status
			// @Description  Requires admin role.
			// @Tags         system
			// @Produce      json
			// @Success      200 {object} apidocs.UpgradeStatus
			// @Failure      401 {object} apidocs.ProblemDetail
			// @Failure      403 {object} apidocs.ProblemDetail
			// @Security     BearerAuth
			// @Router       /system/upgrade/status [get]
			protected.GET("/system/upgrade/status", middleware.RequireRole("admin"), func(c *gin.Context) {
				c.JSON(200, deps.Services.Version.GetUpgradeStatus())
			})
			// clearUpgradeStatus godoc
			// @Summary      Clear upgrade status
			// @Description  Requires admin role.
			// @Tags         system
			// @Produce      json
			// @Success      200 {object} apidocs.MessageResponse
			// @Failure      401 {object} apidocs.ProblemDetail
			// @Failure      403 {object} apidocs.ProblemDetail
			// @Security     BearerAuth
			// @Router       /system/upgrade/status [delete]
			protected.DELETE("/system/upgrade/status", middleware.RequireRole("admin"), func(c *gin.Context) {
				deps.Services.Version.ClearUpgradeStatus()
				c.JSON(200, gin.H{"message": "upgrade status cleared"})
			})

			// System Backups (admin only)
			sysBackup := v1.NewSystemBackupHandler(deps.Services.SystemBackup)
			adminOnly.GET("/system/backup/config", sysBackup.GetConfig)
			adminOnly.PUT("/system/backup/config", sysBackup.SaveConfig)
			adminOnly.POST("/system/backup/trigger", sysBackup.TriggerBackup)
			adminOnly.GET("/system/backup/list", sysBackup.ListBackups)

			// Shared Resources (admin only)
			resourceHandler := v1.NewResourceHandler(deps.Services.Resource)
			adminOnly.GET("/resources", resourceHandler.List)
			adminOnly.POST("/resources", resourceHandler.Create)
			adminOnly.POST("/resources/generate-ssh-key", resourceHandler.GenerateSSHKey)
			adminOnly.GET("/resources/:id/repos", resourceHandler.ListRepos)
			adminOnly.GET("/resources/:id/buckets", resourceHandler.ListBuckets)
			adminOnly.PATCH("/resources/:id", resourceHandler.Update)
			adminOnly.DELETE("/resources/:id", resourceHandler.Delete)
			adminOnly.POST("/resources/:id/test", resourceHandler.TestConnection)

			dnsHandler := v1.NewDNSHandler(deps.Services.Resource)
			adminOnly.GET("/resources/:id/dns/zones", dnsHandler.ListZones)
			adminOnly.GET("/resources/:id/dns/records", dnsHandler.ListRecords)
			adminOnly.POST("/resources/:id/dns/records", dnsHandler.UpsertRecord)
			// POST (not DELETE) because the delete spec travels in the request
			// body; many proxies drop bodies on DELETE requests.
			adminOnly.POST("/resources/:id/dns/records/delete", dnsHandler.DeleteRecord)
		}
	}

	return r
}
