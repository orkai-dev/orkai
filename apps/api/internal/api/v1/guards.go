package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Guards builds reusable route guards that enforce project-scoped access. Each
// guard resolves the target resource to its owning project and delegates the
// allow/deny decision to the Authz service.
type Guards struct {
	store store.Store
	authz *service.Authz
}

func NewGuards(s store.Store, authz *service.Authz) *Guards {
	return &Guards{store: s, authz: authz}
}

// ensureProjectAccess checks access to a project resolved from a request body
// (e.g. resource-creation endpoints where the project ID is in the payload).
// It writes a 403 response and returns false when access is denied.
func ensureProjectAccess(c *gin.Context, authz *service.Authz, projectID uuid.UUID) bool {
	ok, err := authz.CanAccessProject(
		c.Request.Context(),
		middleware.GetUserID(c),
		middleware.GetUserRole(c),
		middleware.GetOrgID(c),
		projectID,
	)
	if err != nil || !ok {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return false
	}
	return true
}

// checkProject runs the access decision for a resolved project ID.
func (g *Guards) checkProject(c *gin.Context, projectID uuid.UUID) {
	ok, err := g.authz.CanAccessProject(
		c.Request.Context(),
		middleware.GetUserID(c),
		middleware.GetUserRole(c),
		middleware.GetOrgID(c),
		projectID,
	)
	if err != nil || !ok {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		c.Abort()
		return
	}
	c.Next()
}

// Project guards a route whose :param is a project ID.
func (g *Guards) Project(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
			c.Abort()
			return
		}
		g.checkProject(c, id)
	}
}

// App guards a route whose :param is an application ID.
func (g *Guards) App(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
			c.Abort()
			return
		}
		app, err := g.store.Applications().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("application not found"))
			c.Abort()
			return
		}
		g.checkProject(c, app.ProjectID)
	}
}

// Database guards a route whose :param is a managed database ID.
func (g *Guards) Database(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
			c.Abort()
			return
		}
		db, err := g.store.ManagedDatabases().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("database not found"))
			c.Abort()
			return
		}
		g.checkProject(c, db.ProjectID)
	}
}

// Page guards a route whose :param is a page ID.
func (g *Guards) Page(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid page ID"))
			c.Abort()
			return
		}
		page, err := g.store.Pages().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("page not found"))
			c.Abort()
			return
		}
		g.checkProject(c, page.ProjectID)
	}
}

// Worker guards a route whose :param is a worker ID.
func (g *Guards) Worker(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
			c.Abort()
			return
		}
		worker, err := g.store.Workers().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("worker not found"))
			c.Abort()
			return
		}
		g.checkProject(c, worker.ProjectID)
	}
}

// CronJob guards a route whose :param is a cron job ID.
func (g *Guards) CronJob(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
			c.Abort()
			return
		}
		cj, err := g.store.CronJobs().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("cronjob not found"))
			c.Abort()
			return
		}
		g.checkProject(c, cj.ProjectID)
	}
}

// Deployment guards a route whose :param is a deployment ID (resolved via app).
func (g *Guards) Deployment(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid deployment ID"))
			c.Abort()
			return
		}
		dep, err := g.store.Deployments().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("deployment not found"))
			c.Abort()
			return
		}
		app, err := g.store.Applications().GetByID(c.Request.Context(), dep.AppID)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("application not found"))
			c.Abort()
			return
		}
		g.checkProject(c, app.ProjectID)
	}
}

// Domain guards a route whose :param is a domain ID (resolved via app).
func (g *Guards) Domain(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param(param))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid domain ID"))
			c.Abort()
			return
		}
		dom, err := g.store.Domains().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("domain not found"))
			c.Abort()
			return
		}
		app, err := g.store.Applications().GetByID(c.Request.Context(), dom.AppID)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("application not found"))
			c.Abort()
			return
		}
		g.checkProject(c, app.ProjectID)
	}
}
