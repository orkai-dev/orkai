package v1

import (
	"fmt"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.DeploymentListResponse
	_ apidocs.MessageResponse
	_ apidocs.ProblemDetail
)

type DeployHandler struct {
	svc   *service.DeployService
	store store.Store
	authz *service.Authz
}

func NewDeployHandler(svc *service.DeployService, s store.Store, authz *service.Authz) *DeployHandler {
	return &DeployHandler{svc: svc, store: s, authz: authz}
}

// scopeProjectIDs returns the project-ID filter to apply for the caller. The
// second return is true when the caller has org-wide access (no filtering).
func (h *DeployHandler) scopeProjectIDs(c *gin.Context) ([]uuid.UUID, bool, error) {
	return h.authz.AccessibleProjectIDs(
		c.Request.Context(),
		middleware.GetUserID(c),
		middleware.GetUserRole(c),
		middleware.GetOrgID(c),
	)
}

// verifyDeploymentOrg checks that a deployment belongs to the caller's org.
func (h *DeployHandler) verifyDeploymentOrg(c *gin.Context, deployID uuid.UUID) (*model.Deployment, error) {
	deploy, err := h.svc.GetByID(c.Request.Context(), deployID)
	if err != nil {
		return nil, err
	}
	app, err := h.store.Applications().GetByID(c.Request.Context(), deploy.AppID)
	if err != nil {
		return nil, err
	}
	project, err := h.store.Projects().GetByID(c.Request.Context(), app.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OrgID != middleware.GetOrgID(c) {
		return nil, fmt.Errorf("access denied")
	}
	return deploy, nil
}

// verifyAppOrg checks that an app belongs to the caller's org.
func (h *DeployHandler) verifyAppOrg(c *gin.Context, appID uuid.UUID) error {
	app, err := h.store.Applications().GetByID(c.Request.Context(), appID)
	if err != nil {
		return err
	}
	project, err := h.store.Projects().GetByID(c.Request.Context(), app.ProjectID)
	if err != nil {
		return err
	}
	if project.OrgID != middleware.GetOrgID(c) {
		return fmt.Errorf("access denied")
	}
	return nil
}

// Trigger godoc
// @Summary      Trigger deployment
// @Tags         deployments
// @Accept       json
// @Produce      json
// @Param        id path string true "Application ID"
// @Param        body body object true "Request body"
// @Success      202 {object} apidocs.Deployment
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/deploy [post]
func (h *DeployHandler) Trigger(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.verifyAppOrg(c, appID); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return
	}

	var body struct {
		ForceBuild bool `json:"force_build"`
	}
	_ = c.ShouldBindJSON(&body)

	input := service.TriggerDeployInput{
		AppID:       appID,
		ForceBuild:  body.ForceBuild,
		TriggerType: "manual",
		TriggeredBy: ptrUUID(middleware.GetUserID(c)),
	}

	deploy, err := h.svc.Trigger(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondAccepted(c, deploy)
}

// Get godoc
// @Summary      Get deployment
// @Tags         deployments
// @Produce      json
// @Param        id path string true "Deployment ID"
// @Success      200 {object} apidocs.Deployment
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /deployments/{id} [get]
func (h *DeployHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid deployment ID"))
		return
	}

	deploy, err := h.verifyDeploymentOrg(c, id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("deployment not found"))
		return
	}

	httputil.RespondOK(c, deploy)
}

// List godoc
// @Summary      List application deployments
// @Tags         deployments
// @Produce      json
// @Param        id path string true "Application ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.DeploymentListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/deployments [get]
func (h *DeployHandler) List(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.verifyAppOrg(c, appID); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return
	}

	params := bindListParams(c)
	deploys, total, err := h.svc.List(c.Request.Context(), appID, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(deploys, params.Page, params.PerPage, total))
}

// ListAll returns deployments across all apps with optional status filter.
// ListAll godoc
// @Summary      List all deployments
// @Tags         deployments
// @Produce      json
// @Param        status query string false "Status filter"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.DeploymentListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /deployments [get]
func (h *DeployHandler) ListAll(c *gin.Context) {
	params := bindListParams(c)
	filter := store.DeploymentListFilter{
		Status: c.Query("status"),
	}

	// Members only see deployments within their teams' projects.
	ids, isAll, err := h.scopeProjectIDs(c)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if !isAll {
		filter.ProjectIDs = ids
	}

	deploys, total, err := h.svc.ListAll(c.Request.Context(), params, filter)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(deploys, params.Page, params.PerPage, total))
}

// ListQueue returns only active (in-progress) deployments.
// ListQueue godoc
// @Summary      List deployment queue
// @Tags         deployments
// @Produce      json
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.DeploymentListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /deployments/queue [get]
func (h *DeployHandler) ListQueue(c *gin.Context) {
	params := bindListParams(c)

	// Members only see deployments within their teams' projects.
	scopeIDs, isAll, err := h.scopeProjectIDs(c)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	// Get queued + building + deploying deployments
	var allDeploys []model.Deployment
	totalCount := 0
	for _, status := range []string{"queued", "building", "deploying"} {
		filter := store.DeploymentListFilter{Status: status}
		if !isAll {
			filter.ProjectIDs = scopeIDs
		}
		deploys, count, err := h.svc.ListAll(c.Request.Context(), store.ListParams{Page: 1, PerPage: 100}, filter)
		if err != nil {
			continue
		}
		allDeploys = append(allDeploys, deploys...)
		totalCount += count
	}

	// Use actual fetched count as total (avoids mismatch when a bucket exceeds 100)
	totalCount = len(allDeploys)

	// Sort by created_at descending (newest first) across all status buckets
	sort.Slice(allDeploys, func(i, j int) bool {
		return allDeploys[i].CreatedAt.After(allDeploys[j].CreatedAt)
	})

	// Apply pagination manually
	start := params.Offset()
	end := start + params.Limit()
	if start > len(allDeploys) {
		start = len(allDeploys)
	}
	if end > len(allDeploys) {
		end = len(allDeploys)
	}

	httputil.RespondOK(c, httputil.NewListResponse(allDeploys[start:end], params.Page, params.PerPage, totalCount))
}

// Cancel godoc
// @Summary      Cancel deployment
// @Tags         deployments
// @Produce      json
// @Param        id path string true "Deployment ID"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /deployments/{id}/cancel [post]
func (h *DeployHandler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid deployment ID"))
		return
	}
	if _, err := h.verifyDeploymentOrg(c, id); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "deployment cancelled"})
}

// Rollback godoc
// @Summary      Rollback deployment
// @Tags         deployments
// @Produce      json
// @Param        id path string true "Deployment ID"
// @Success      202 {object} apidocs.Deployment
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /deployments/{id}/rollback [post]
func (h *DeployHandler) Rollback(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid deployment ID"))
		return
	}
	if _, err := h.verifyDeploymentOrg(c, id); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return
	}

	deploy, err := h.svc.Rollback(c.Request.Context(), id, ptrUUID(middleware.GetUserID(c)))
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondAccepted(c, deploy)
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
