package v1

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

type APIKeyHandler struct {
	svc *service.APIKeyService
}

var _ apidocs.ProblemDetail

func NewAPIKeyHandler(svc *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{svc: svc}
}

// List godoc
// @Summary      List API keys
// @Tags         api-keys
// @Produce      json
// @Success      200 {array} apidocs.APIKey
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /api-keys [get]
func (h *APIKeyHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	keys, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if keys == nil {
		keys = []model.APIKey{}
	}
	httputil.RespondOK(c, keys)
}

// Create godoc
// @Summary      Create API key
// @Tags         api-keys
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {object} object
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /api-keys [post]
func (h *APIKeyHandler) Create(c *gin.Context) {
	var input struct {
		Name      string     `json:"name" binding:"required"`
		Role      string     `json:"role"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	orgID := middleware.GetOrgID(c)
	callerRole := middleware.GetUserRole(c)

	requestedRole := model.RoleMember
	if input.Role != "" {
		requestedRole = model.Role(input.Role)
	}

	result, err := h.svc.Create(
		c.Request.Context(),
		userID,
		orgID,
		callerRole,
		input.Name,
		requestedRole,
		input.ExpiresAt,
	)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

// Revoke godoc
// @Summary      Revoke API key
// @Tags         api-keys
// @Produce      json
// @Param        id path string true "API key ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /api-keys/{id} [delete]
func (h *APIKeyHandler) Revoke(c *gin.Context) {
	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid key id"))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.Revoke(c.Request.Context(), userID, keyID); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "revoked"})
}
