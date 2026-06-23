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
	_ apidocs.MessageResponse
	_ apidocs.NodeListResponse
	_ apidocs.ProblemDetail
)

type NodeHandler struct {
	svc *service.NodeService
}

func NewNodeHandler(svc *service.NodeService) *NodeHandler {
	return &NodeHandler{svc: svc}
}

// List godoc
// @Summary      List server nodes
// @Tags         nodes
// @Produce      json
// @Success      200 {object} apidocs.NodeListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /nodes [get]
func (h *NodeHandler) List(c *gin.Context) {
	nodes, err := h.svc.List(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, nodes)
}

// Create godoc
// @Summary      Create server node
// @Tags         nodes
// @Accept       json
// @Produce      json
// @Param        body body apidocs.CreateNodeInput true "Node configuration"
// @Success      201 {object} apidocs.ServerNode
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /nodes [post]
func (h *NodeHandler) Create(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var input service.CreateNodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	node, err := h.svc.Create(c.Request.Context(), orgID, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondCreated(c, node, "")
}

// Initialize godoc
// @Summary      Initialize server node
// @Tags         nodes
// @Produce      json
// @Param        id path string true "Node ID"
// @Success      202 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /nodes/{id}/initialize [post]
func (h *NodeHandler) Initialize(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid node ID"))
		return
	}
	if err := h.svc.Initialize(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondAccepted(c, gin.H{"message": "initialization started"})
}

// Delete godoc
// @Summary      Delete server node
// @Tags         nodes
// @Produce      json
// @Param        id path string true "Node ID"
// @Success      204
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /nodes/{id} [delete]
func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid node ID"))
		return
	}
	orgID := middleware.GetOrgID(c)
	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondNoContent(c)
}
