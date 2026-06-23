package v1

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ProblemDetail
	_ apidocs.ProjectListResponse
)

type ProjectHandler struct {
	svc *service.ProjectService
}

func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

// List godoc
// @Summary      List projects
// @Tags         projects
// @Produce      json
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.ProjectListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects [get]
func (h *ProjectHandler) List(c *gin.Context) {
	params := bindListParams(c)
	orgID := middleware.GetOrgID(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	projects, total, err := h.svc.ListForUser(c.Request.Context(), userID, role, orgID, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(projects, params.Page, params.PerPage, total))
}

// Create godoc
// @Summary      Create project
// @Tags         projects
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body apidocs.CreateProjectInput true "Request body"
// @Success      201 {object} apidocs.Project
// @Failure      403 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects [post]
func (h *ProjectHandler) Create(c *gin.Context) {
	var input service.CreateProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	project, err := h.svc.Create(c.Request.Context(), orgID, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondCreated(c, project, fmt.Sprintf("/api/v1/projects/%s", project.ID))
}

// Get godoc
// @Summary      Get project
// @Tags         projects
// @Produce      json
// @Param        id path string true "Project ID"
// @Success      200 {object} apidocs.Project
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id} [get]
func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	project, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("project not found"))
		return
	}

	httputil.RespondOK(c, project)
}

// Update godoc
// @Summary      Update project
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        id path string true "Project ID"
// @Param        body body apidocs.UpdateProjectInput true "Request body"
// @Success      200 {object} apidocs.Project
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id} [patch]
func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	var input service.UpdateProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	project, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, project)
}

// Delete godoc
// @Summary      Delete project
// @Tags         projects
// @Produce      json
// @Param        id path string true "Project ID"
// @Success      204
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id} [delete]
func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondNoContent(c)
}

// UpdateEnv godoc
// @Summary      Update project environment variables
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        id path string true "Project ID"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.Project
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id}/env [put]
func (h *ProjectHandler) UpdateEnv(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}
	var input struct {
		EnvVars map[string]string `json:"env_vars" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	project, err := h.svc.UpdateEnvVars(c.Request.Context(), id, input.EnvVars)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, project)
}

// bindListParams extracts pagination params from query string.
func bindListParams(c *gin.Context) store.ListParams {
	params := store.DefaultListParams()
	type query struct {
		Page    int `form:"page"`
		PerPage int `form:"per_page"`
	}
	var q query
	if err := c.ShouldBindQuery(&q); err == nil {
		if q.Page > 0 {
			params.Page = q.Page
		}
		if q.PerPage > 0 && q.PerPage <= 100 {
			params.PerPage = q.PerPage
		}
	}
	return params
}
