package v1

import (
	"fmt"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.CronJobListResponse
	_ apidocs.ProblemDetail
)

type CronJobHandler struct {
	svc   *service.CronJobService
	authz *service.Authz
}

func NewCronJobHandler(svc *service.CronJobService, authz *service.Authz) *CronJobHandler {
	return &CronJobHandler{svc: svc, authz: authz}
}

// ListByProject godoc
// @Summary      List cron jobs in project
// @Tags         cronjobs
// @Produce      json
// @Param        id path string true "Project ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.CronJobListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id}/cronjobs [get]
func (h *CronJobHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}
	params := bindListParams(c)
	jobs, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(jobs, params.Page, params.PerPage, total))
}

// Create godoc
// @Summary      Create cron job
// @Tags         cronjobs
// @Accept       json
// @Produce      json
// @Param        body body apidocs.CreateCronJobInput true "Request body"
// @Success      201 {object} apidocs.CronJob
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cronjobs [post]
func (h *CronJobHandler) Create(c *gin.Context) {
	var input service.CreateCronJobInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	// Verify the caller may create resources in the target project
	if !ensureProjectAccess(c, h.authz, input.ProjectID) {
		return
	}
	cj, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondCreated(c, cj, fmt.Sprintf("/api/v1/cronjobs/%s", cj.ID))
}

// Get godoc
// @Summary      Get cron job
// @Tags         cronjobs
// @Produce      json
// @Param        id path string true "Cron job ID"
// @Success      200 {object} apidocs.CronJob
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cronjobs/{id} [get]
func (h *CronJobHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	cj, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("cronjob not found"))
		return
	}
	httputil.RespondOK(c, cj)
}

// Update godoc
// @Summary      Update cron job
// @Tags         cronjobs
// @Accept       json
// @Produce      json
// @Param        id path string true "Cron job ID"
// @Param        body body apidocs.UpdateCronJobInput true "Request body"
// @Success      200 {object} apidocs.CronJob
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cronjobs/{id} [patch]
func (h *CronJobHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	var input service.UpdateCronJobInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	cj, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, cj)
}

// Delete godoc
// @Summary      Delete cron job
// @Tags         cronjobs
// @Produce      json
// @Param        id path string true "Cron job ID"
// @Success      204
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cronjobs/{id} [delete]
func (h *CronJobHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondNoContent(c)
}

// Trigger godoc
// @Summary      Trigger cron job run
// @Tags         cronjobs
// @Produce      json
// @Param        id path string true "Cron job ID"
// @Success      202 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cronjobs/{id}/trigger [post]
func (h *CronJobHandler) Trigger(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	run, err := h.svc.Trigger(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondAccepted(c, run)
}

// ListRuns godoc
// @Summary      List cron job runs
// @Tags         cronjobs
// @Produce      json
// @Param        id path string true "Cron job ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cronjobs/{id}/runs [get]
func (h *CronJobHandler) ListRuns(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	params := bindListParams(c)
	runs, total, err := h.svc.ListRuns(c.Request.Context(), id, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(runs, params.Page, params.PerPage, total))
}
