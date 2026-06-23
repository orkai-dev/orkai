package v1

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type WorkerHandler struct {
	svc       *service.WorkerService
	deploySvc *service.WorkerDeployService
	store     store.Store
	authz     *service.Authz
}

func NewWorkerHandler(svc *service.WorkerService, deploySvc *service.WorkerDeployService, s store.Store, authz *service.Authz) *WorkerHandler {
	return &WorkerHandler{svc: svc, deploySvc: deploySvc, store: s, authz: authz}
}

func workerErr(err error) error {
	if _, ok := err.(*apierr.ProblemDetail); ok {
		return err
	}
	return apierr.ErrBadRequest.WithDetail(err.Error())
}

// ListByProject lists workers in a project.
func (h *WorkerHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}
	params := bindListParams(c)
	items, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(items, params.Page, params.PerPage, total))
}

// ListAll lists all workers accessible to the caller.
func (h *WorkerHandler) ListAll(c *gin.Context) {
	params := bindListParams(c)
	filter := store.WorkerListFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
	}

	ids, isAll, err := h.authz.AccessibleProjectIDs(
		c.Request.Context(),
		middleware.GetUserID(c),
		middleware.GetUserRole(c),
		middleware.GetOrgID(c),
	)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if !isAll {
		filter.ProjectIDs = ids
	}

	items, total, err := h.svc.ListAll(c.Request.Context(), params, filter)
	if err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(items, params.Page, params.PerPage, total))
}

// Create creates a worker.
func (h *WorkerHandler) Create(c *gin.Context) {
	var input service.CreateWorkerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if !ensureProjectAccess(c, h.authz, input.ProjectID) {
		return
	}
	worker, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondCreated(c, worker, fmt.Sprintf("/api/v1/workers/%s", worker.ID))
}

// Get returns a worker.
func (h *WorkerHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
		return
	}
	worker, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("worker not found"))
		return
	}
	httputil.RespondOK(c, worker)
}

// Update updates a worker.
func (h *WorkerHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
		return
	}
	var input service.UpdateWorkerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	worker, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondOK(c, worker)
}

// Delete deletes a worker.
func (h *WorkerHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondNoContent(c)
}

// Deploy triggers a worker deployment.
func (h *WorkerHandler) Deploy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
		return
	}
	dep, err := h.deploySvc.Trigger(c.Request.Context(), id, "manual")
	if err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondCreated(c, dep, fmt.Sprintf("/api/v1/workers/%s/deployments/%s", id, dep.ID))
}

// ConfirmR2 approves the pre-existing R2 buckets flagged by the latest deploy
// and re-triggers the deployment.
func (h *WorkerHandler) ConfirmR2(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
		return
	}
	dep, err := h.deploySvc.ConfirmR2(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondCreated(c, dep, fmt.Sprintf("/api/v1/workers/%s/deployments/%s", id, dep.ID))
}

// ListDeployments returns a worker's deployment history.
func (h *WorkerHandler) ListDeployments(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
		return
	}
	params := bindListParams(c)
	items, total, err := h.svc.ListDeployments(c.Request.Context(), id, params)
	if err != nil {
		httputil.RespondError(c, workerErr(err))
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(items, params.Page, params.PerPage, total))
}

// GetDeployment returns a single worker deployment.
func (h *WorkerHandler) GetDeployment(c *gin.Context) {
	workerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid worker ID"))
		return
	}
	deployID, err := uuid.Parse(c.Param("deployId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid deployment ID"))
		return
	}
	dep, err := h.svc.GetDeployment(c.Request.Context(), deployID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("deployment not found"))
		return
	}
	// guards.Worker only authorizes :id; ensure the deployment actually belongs
	// to that worker so a UUID for another worker/org can't be read (IDOR).
	if dep.WorkerID != workerID {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("deployment not found"))
		return
	}
	httputil.RespondOK(c, dep)
}
