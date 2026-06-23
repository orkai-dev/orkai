package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ProblemDetail
	_ apidocs.TemplateListResponse
)

type TemplateHandler struct {
	svc *service.TemplateService
}

func NewTemplateHandler(svc *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

// List godoc
// @Summary      List templates
// @Tags         templates
// @Produce      json
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.TemplateListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /templates [get]
func (h *TemplateHandler) List(c *gin.Context) {
	params := bindListParams(c)
	templates, total, err := h.svc.List(c.Request.Context(), params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(templates, params.Page, params.PerPage, total))
}

// Get godoc
// @Summary      Get template
// @Tags         templates
// @Produce      json
// @Param        id path string true "Template ID"
// @Success      200 {object} apidocs.Template
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /templates/{id} [get]
func (h *TemplateHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid template ID"))
		return
	}

	tpl, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("template not found"))
		return
	}

	httputil.RespondOK(c, tpl)
}
