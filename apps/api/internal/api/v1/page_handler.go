package v1

import (
	"fmt"

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
	_ apidocs.PageListResponse
	_ apidocs.ProblemDetail
)

type PageHandler struct {
	svc       *service.PageService
	deploySvc *service.PageDeployService
	store     store.Store
	authz     *service.Authz
}

func NewPageHandler(svc *service.PageService, deploySvc *service.PageDeployService, s store.Store, authz *service.Authz) *PageHandler {
	return &PageHandler{svc: svc, deploySvc: deploySvc, store: s, authz: authz}
}

// pageErr wraps a service error as a ProblemDetail if it isn't one already.
func pageErr(err error) error {
	if _, ok := err.(*apierr.ProblemDetail); ok {
		return err
	}
	return apierr.ErrBadRequest.WithDetail(err.Error())
}

// ListByProject godoc
// @Summary      List pages in project
// @Tags         pages
// @Produce      json
// @Param        id path string true "Project ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.PageListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id}/pages [get]
func (h *PageHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}
	params := bindListParams(c)
	items, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, pageErr(err))
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(items, params.Page, params.PerPage, total))
}

// ListAll godoc
// @Summary      List all pages
// @Tags         pages
// @Produce      json
// @Param        search query string false "Search term"
// @Param        status query string false "Status filter"
// @Param        provider query string false "Provider filter (aws_cloudfront, cloudflare_pages)"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.PageListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages [get]
func (h *PageHandler) ListAll(c *gin.Context) {
	params := bindListParams(c)
	provider := c.Query("provider")
	if provider != "" &&
		provider != string(model.PageProviderAWSCloudFront) &&
		provider != string(model.PageProviderCloudflarePages) {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("provider must be aws_cloudfront or cloudflare_pages"))
		return
	}
	filter := store.PageListFilter{
		Search:   c.Query("search"),
		Status:   c.Query("status"),
		Provider: provider,
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
		httputil.RespondError(c, pageErr(err))
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(items, params.Page, params.PerPage, total))
}

// Create godoc
// @Summary      Create page
// @Tags         pages
// @Accept       json
// @Produce      json
// @Param        body body apidocs.CreatePageInput true "Request body"
// @Success      201 {object} apidocs.Page
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages [post]
func (h *PageHandler) Create(c *gin.Context) {
	var input service.CreatePageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if !ensureProjectAccess(c, h.authz, input.ProjectID) {
		return
	}
	page, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, pageErr(err))
		return
	}
	httputil.RespondCreated(c, page, fmt.Sprintf("/api/v1/pages/%s", page.ID))
}

// Get godoc
// @Summary      Get page
// @Tags         pages
// @Produce      json
// @Param        id path string true "Page ID"
// @Success      200 {object} apidocs.Page
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages/{id} [get]
func (h *PageHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid page ID"))
		return
	}
	page, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("page not found"))
		return
	}
	httputil.RespondOK(c, page)
}

// Update godoc
// @Summary      Update page
// @Tags         pages
// @Accept       json
// @Produce      json
// @Param        id path string true "Page ID"
// @Param        body body apidocs.UpdatePageInput true "Request body"
// @Success      200 {object} apidocs.Page
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages/{id} [patch]
func (h *PageHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid page ID"))
		return
	}
	var input service.UpdatePageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	page, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, pageErr(err))
		return
	}
	httputil.RespondOK(c, page)
}

// Delete godoc
// @Summary      Delete page
// @Tags         pages
// @Produce      json
// @Param        id path string true "Page ID"
// @Success      204
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages/{id} [delete]
func (h *PageHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid page ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, pageErr(err))
		return
	}
	httputil.RespondNoContent(c)
}

// Deploy godoc
// @Summary      Deploy page
// @Tags         pages
// @Produce      json
// @Param        id path string true "Page ID"
// @Success      201 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages/{id}/deploy [post]
func (h *PageHandler) Deploy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid page ID"))
		return
	}
	dep, err := h.deploySvc.Trigger(c.Request.Context(), id, "manual")
	if err != nil {
		httputil.RespondError(c, pageErr(err))
		return
	}
	httputil.RespondCreated(c, dep, fmt.Sprintf("/api/v1/pages/%s/deployments/%s", id, dep.ID))
}

// ListDeployments godoc
// @Summary      List page deployments
// @Tags         pages
// @Produce      json
// @Param        id path string true "Page ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages/{id}/deployments [get]
func (h *PageHandler) ListDeployments(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid page ID"))
		return
	}
	params := bindListParams(c)
	items, total, err := h.svc.ListDeployments(c.Request.Context(), id, params)
	if err != nil {
		httputil.RespondError(c, pageErr(err))
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(items, params.Page, params.PerPage, total))
}

// GetDeployment godoc
// @Summary      Get page deployment
// @Tags         pages
// @Produce      json
// @Param        id path string true "Page ID"
// @Param        deployId path string true "Deployment ID"
// @Success      200 {object} object
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /pages/{id}/deployments/{deployId} [get]
func (h *PageHandler) GetDeployment(c *gin.Context) {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid page ID"))
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
	// guards.Page only authorizes :id; ensure the deployment actually belongs to
	// that page so a UUID for another page/org can't be read (IDOR).
	if dep.PageID != pageID {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("deployment not found"))
		return
	}
	httputil.RespondOK(c, dep)
}
