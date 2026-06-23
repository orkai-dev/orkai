package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ProblemDetail
	_ apidocs.ResourceListResponse
)

type ResourceHandler struct {
	svc *service.ResourceService
}

func NewResourceHandler(svc *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

// List godoc
// @Summary      List shared resources
// @Tags         resources
// @Description  Requires admin role.
// @Produce      json
// @Param        type query string false "Resource type filter"
// @Success      200 {object} apidocs.ResourceListResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources [get]
func (h *ResourceHandler) List(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	resourceType := c.Query("type")
	resources, err := h.svc.List(c.Request.Context(), orgID, resourceType)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	for i := range resources {
		resources[i] = service.RedactResourceConfig(resources[i])
	}
	httputil.RespondList(c, resources)
}

// Create godoc
// @Summary      Create shared resource
// @Tags         resources
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body apidocs.CreateResourceInput true "Request body"
// @Success      201 {object} apidocs.SharedResource
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources [post]
func (h *ResourceHandler) Create(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var input service.CreateResourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	resource, err := h.svc.Create(c.Request.Context(), orgID, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	redacted := service.RedactResourceConfig(*resource)
	httputil.RespondCreated(c, &redacted, "")
}

// Update godoc
// @Summary      Update shared resource
// @Tags         resources
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        id path string true "Resource ID"
// @Param        body body apidocs.UpdateResourceInput true "Request body"
// @Success      200 {object} apidocs.SharedResource
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id} [patch]
func (h *ResourceHandler) Update(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	var input service.UpdateResourceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	resource, err := h.svc.Update(c.Request.Context(), orgID, id, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	redacted := service.RedactResourceConfig(*resource)
	httputil.RespondOK(c, &redacted)
}

// Delete godoc
// @Summary      Delete shared resource
// @Tags         resources
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Resource ID"
// @Success      204
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id} [delete]
func (h *ResourceHandler) Delete(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondNoContent(c)
}

// TestConnection godoc
// @Summary      Test resource connection
// @Tags         resources
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Resource ID"
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/test [post]
func (h *ResourceHandler) TestConnection(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	ok, msg, err := h.svc.TestConnection(c.Request.Context(), orgID, id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"success": ok, "message": msg})
}

// ListRepos godoc
// @Summary      List Git repositories
// @Tags         resources
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Resource ID"
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/repos [get]
func (h *ResourceHandler) ListRepos(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	repos, err := h.svc.ListRepos(c.Request.Context(), orgID, id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if repos == nil {
		repos = []service.GitRepo{}
	}
	httputil.RespondOK(c, repos)
}

// SearchRepos godoc
// @Summary      Search Git repositories
// @Tags         resources
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Resource ID"
// @Param        q query string false "Search query"
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/repos/search [get]
func (h *ResourceHandler) SearchRepos(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	repos, err := h.svc.SearchRepos(c.Request.Context(), orgID, id, c.Query("q"))
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if repos == nil {
		repos = []service.GitRepo{}
	}
	httputil.RespondOK(c, repos)
}

// ListBuckets returns the S3 buckets visible to a cloud_account resource.
// ListBuckets godoc
// @Summary      List S3 buckets
// @Tags         resources
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Resource ID"
// @Success      200 {array} string
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/buckets [get]
func (h *ResourceHandler) ListBuckets(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	buckets, err := h.svc.ListBuckets(c.Request.Context(), orgID, id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if buckets == nil {
		buckets = []string{}
	}
	httputil.RespondOK(c, buckets)
}

// GenerateSSHKey godoc
// @Summary      Generate SSH key resource
// @Tags         resources
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      201 {object} apidocs.SharedResource
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/generate-ssh-key [post]
func (h *ResourceHandler) GenerateSSHKey(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var input struct {
		Algorithm string `json:"algorithm"` // ed25519 | rsa-4096
		Name      string `json:"name"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if input.Algorithm == "" {
		input.Algorithm = "ed25519"
	}
	resource, err := h.svc.GenerateSSHKey(c.Request.Context(), orgID, input.Algorithm, input.Name)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	redacted := service.RedactResourceConfig(*resource)
	httputil.RespondCreated(c, &redacted, "")
}
