package v1

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ProblemDetail
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// ListChannels godoc
// @Summary      List notification channels
// @Tags         notifications
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /notifications/channels [get]
func (h *NotificationHandler) ListChannels(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	channels, err := h.svc.ListChannels(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, channels)
}

// ListEvents godoc
// @Summary      List notification event types
// @Tags         notifications
// @Description  Requires admin role. Returns all event types with labels and categories for the settings UI.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /notifications/events [get]
func (h *NotificationHandler) ListEvents(c *gin.Context) {
	httputil.RespondOK(c, model.AllNotifyEventInfos())
}

// SaveChannel godoc
// @Summary      Save notification channel
// @Tags         notifications
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /notifications/channels [put]
func (h *NotificationHandler) SaveChannel(c *gin.Context) {
	var input struct {
		Type    string          `json:"type" binding:"required"`
		Enabled bool            `json:"enabled"`
		Config  json.RawMessage `json:"config"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.SaveChannel(c.Request.Context(), orgID, input.Type, input.Enabled, input.Config); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"status": "saved"})
}

// TestChannel godoc
// @Summary      Test notification channel
// @Tags         notifications
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /notifications/test [post]
func (h *NotificationHandler) TestChannel(c *gin.Context) {
	var input struct {
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.TestChannel(c.Request.Context(), orgID, input.Type); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"status": "sent"})
}

// GetSMTPConfig godoc
// @Summary      Get SMTP configuration
// @Tags         notifications
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} apidocs.SMTPConfig
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /settings/smtp [get]
func (h *NotificationHandler) GetSMTPConfig(c *gin.Context) {
	cfg, err := h.svc.GetSMTPConfig(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	// Mask password in response
	masked := *cfg
	if masked.Password != "" {
		masked.Password = model.SettingSecretMask
	}
	httputil.RespondOK(c, masked)
}

// SaveSMTPConfig godoc
// @Summary      Save SMTP configuration
// @Tags         notifications
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body apidocs.SMTPConfig true "Request body"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /settings/smtp [put]
func (h *NotificationHandler) SaveSMTPConfig(c *gin.Context) {
	var input service.SMTPConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.SaveSMTPConfig(c.Request.Context(), &input); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"status": "saved"})
}

// TestSMTP godoc
// @Summary      Test SMTP configuration
// @Tags         notifications
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /settings/smtp/test [post]
func (h *NotificationHandler) TestSMTP(c *gin.Context) {
	if err := h.svc.TestSMTP(c.Request.Context()); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"status": "sent"})
}
