package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.MessageResponse
	_ apidocs.ProblemDetail
)

type AuthHandler struct {
	svc      *service.AuthService
	sessions middleware.SessionInvalidator
}

func NewAuthHandler(svc *service.AuthService, sessions middleware.SessionInvalidator) *AuthHandler {
	return &AuthHandler{svc: svc, sessions: sessions}
}

// SetupStatus godoc
// @Summary      Get setup status
// @Description  Returns whether the system has been initialized.
// @Tags         auth
// @Produce      json
// @Success      200 {object} apidocs.SetupStatus
// @Failure      500 {object} apidocs.ProblemDetail
// @Router       /auth/setup-status [get]
func (h *AuthHandler) SetupStatus(c *gin.Context) {
	status, err := h.svc.GetSetupStatus(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, status)
}

// Register godoc
// @Summary      Register first admin user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body apidocs.RegisterInput true "Registration payload"
// @Success      201 {object} apidocs.AuthResult
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	result, err := h.svc.Register(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondCreated(c, result, "")
}

// Login godoc
// @Summary      Login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body apidocs.LoginInput true "Login credentials"
// @Success      200 {object} apidocs.AuthResult
// @Failure      401 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	result, err := h.svc.Login(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.ErrUnauthorized.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, result)
}

// Refresh godoc
// @Summary      Refresh access token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body apidocs.RefreshInput true "Refresh token"
// @Success      200 {object} apidocs.AuthResult
// @Failure      401 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var input service.RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	result, err := h.svc.Refresh(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnauthorized, apierr.ErrUnauthorized.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, result)
}

// Logout godoc
// @Summary      Logout
// @Tags         auth
// @Produce      json
// @Success      200 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Failure      500 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if err := h.svc.Logout(c.Request.Context(), userID); err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}
	if h.sessions != nil {
		h.sessions.Invalidate(userID)
	}

	httputil.RespondOK(c, gin.H{"message": "logged out successfully"})
}

// Me godoc
// @Summary      Get current user
// @Tags         auth
// @Produce      json
// @Success      200 {object} apidocs.User
// @Failure      401 {object} apidocs.ProblemDetail
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.svc.GetUser(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("user not found"))
		return
	}

	httputil.RespondOK(c, user)
}

// UpdateProfile godoc
// @Summary      Update profile
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body apidocs.UpdateProfileInput true "Request body"
// @Success      200 {object} apidocs.User
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/profile [patch]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var input service.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, user)
}

// ChangePassword godoc
// @Summary      Change password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body apidocs.ChangePasswordInput true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.ChangePassword(c.Request.Context(), userID, input); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if h.sessions != nil {
		h.sessions.Invalidate(userID)
	}

	httputil.RespondOK(c, gin.H{"message": "password changed successfully"})
}

// ListAvatars godoc
// @Summary      List avatar options
// @Tags         auth
// @Produce      json
// @Success      200 {array} string
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/avatars [get]
func (h *AuthHandler) ListAvatars(c *gin.Context) {
	httputil.RespondOK(c, h.svc.ListAvatars())
}

// Setup2FA godoc
// @Summary      Begin 2FA setup
// @Tags         auth
// @Produce      json
// @Success      200 {object} object
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/2fa/setup [post]
func (h *AuthHandler) Setup2FA(c *gin.Context) {
	userID := middleware.GetUserID(c)
	result, err := h.svc.Setup2FA(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, result)
}

// Verify2FA godoc
// @Summary      Verify and enable 2FA
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body apidocs.Verify2FAInput true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/2fa/verify [post]
func (h *AuthHandler) Verify2FA(c *gin.Context) {
	var input service.Verify2FAInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.Verify2FA(c.Request.Context(), userID, input.Code); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if h.sessions != nil {
		h.sessions.Invalidate(userID)
	}

	httputil.RespondOK(c, gin.H{"message": "2FA enabled successfully"})
}

// Disable2FA godoc
// @Summary      Disable 2FA
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body apidocs.Verify2FAInput true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /auth/2fa/disable [post]
func (h *AuthHandler) Disable2FA(c *gin.Context) {
	var input service.Verify2FAInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.Disable2FA(c.Request.Context(), userID, input.Code); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if h.sessions != nil {
		h.sessions.Invalidate(userID)
	}

	httputil.RespondOK(c, gin.H{"message": "2FA disabled successfully"})
}
