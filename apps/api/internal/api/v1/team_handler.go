package v1

import (
	"fmt"
	"strings"

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
	_ apidocs.ProblemDetail
)

type TeamHandler struct {
	svc      *service.TeamService
	notifSvc *service.NotificationService
	appURL   string
	sessions middleware.SessionInvalidator
}

func NewTeamHandler(svc *service.TeamService, notifSvc *service.NotificationService, appURL string, sessions middleware.SessionInvalidator) *TeamHandler {
	return &TeamHandler{svc: svc, notifSvc: notifSvc, appURL: appURL, sessions: sessions}
}

// ListMembers returns all members of the current organization.
// ListMembers godoc
// @Summary      List organization members
// @Tags         team
// @Produce      json
// @Success      200 {array} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /team/members [get]
func (h *TeamHandler) ListMembers(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	members, err := h.svc.ListMembers(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}

	httputil.RespondList(c, members)
}

// UpdateMemberRole changes a member's role within the organization.
// UpdateMemberRole godoc
// @Summary      Update member role
// @Tags         team
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        id path string true "Member user ID"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /team/members/{id}/role [patch]
func (h *TeamHandler) UpdateMemberRole(c *gin.Context) {
	memberID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid member id"))
		return
	}

	var input struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	// Prevent self-modification
	requesterID := middleware.GetUserID(c)
	if requesterID == memberID {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("cannot change your own role"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.UpdateMemberRole(c.Request.Context(), orgID, memberID, input.Role); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if h.sessions != nil {
		h.sessions.Invalidate(memberID)
	}

	httputil.RespondOK(c, gin.H{"message": "member role updated"})
}

// RemoveMember removes a member from the organization.
// RemoveMember godoc
// @Summary      Remove organization member
// @Tags         team
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Member user ID"
// @Success      204
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /team/members/{id} [delete]
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	memberID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid member id"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.RemoveMember(c.Request.Context(), orgID, memberID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if h.sessions != nil {
		h.sessions.Invalidate(memberID)
	}

	httputil.RespondNoContent(c)
}

// InviteMember creates an invitation for a new member.
// InviteMember godoc
// @Summary      Invite organization member
// @Tags         team
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      201 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /team/invitations [post]
func (h *TeamHandler) InviteMember(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
		Role  string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	userID := middleware.GetUserID(c)

	inv, err := h.svc.InviteMember(c.Request.Context(), orgID, userID, input.Email, input.Role)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	if inv.Created {
		httputil.RespondCreated(c, gin.H{
			"created": true,
			"user":    inv.User,
		}, "")
		return
	}

	inviteURL := fmt.Sprintf("%s/auth/invite?token=%s", strings.TrimRight(h.appURL, "/"), inv.Invitation.Token)

	// Best-effort: send invitation email if SMTP is configured
	emailSent := false
	if err := h.notifSvc.SendInvitationEmail(c.Request.Context(), input.Email, input.Role, inviteURL); err == nil {
		emailSent = true
	}

	httputil.RespondCreated(c, gin.H{
		"invitation": inv.Invitation,
		"invite_url": inviteURL,
		"email_sent": emailSent,
	}, "")
}

// AcceptInvitation accepts a pending invitation.
// AcceptInvitation godoc
// @Summary      Accept team invitation
// @Tags         team
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /team/invitations/accept [post]
func (h *TeamHandler) AcceptInvitation(c *gin.Context) {
	var input struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.svc.AcceptInvitation(c.Request.Context(), input.Token, userID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "invitation accepted"})
}

// ListInvitations returns all invitations for the current organization.
// ListInvitations godoc
// @Summary      List pending invitations
// @Tags         team
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /team/invitations [get]
func (h *TeamHandler) ListInvitations(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	invitations, err := h.svc.ListInvitations(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}

	httputil.RespondList(c, invitations)
}

// CancelInvitation deletes a pending invitation.
// CancelInvitation godoc
// @Summary      Cancel invitation
// @Tags         team
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Invitation ID"
// @Success      204
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /team/invitations/{id} [delete]
func (h *TeamHandler) CancelInvitation(c *gin.Context) {
	invID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid invitation id"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.CancelInvitation(c.Request.Context(), orgID, invID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondNoContent(c)
}

// ListTeams returns all teams in the current organization.
// ListTeams godoc
// @Summary      List teams
// @Tags         team
// @Produce      json
// @Success      200 {array} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /teams [get]
func (h *TeamHandler) ListTeams(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	teams, err := h.svc.ListTeams(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail(err.Error()))
		return
	}
	httputil.RespondList(c, teams)
}

// CreateTeam creates a new team within the organization.
// CreateTeam godoc
// @Summary      Create team
// @Tags         team
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      201 {object} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /teams [post]
func (h *TeamHandler) CreateTeam(c *gin.Context) {
	var input struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	team, err := h.svc.CreateTeam(c.Request.Context(), orgID, input.Name, input.Description)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondCreated(c, team, fmt.Sprintf("/api/v1/teams/%s", team.ID))
}

// DeleteTeam removes a team (must have no projects).
// DeleteTeam godoc
// @Summary      Delete team
// @Tags         team
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Team ID"
// @Success      204
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /teams/{id} [delete]
func (h *TeamHandler) DeleteTeam(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid team id"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.DeleteTeam(c.Request.Context(), orgID, teamID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondNoContent(c)
}

// ListTeamMembers returns the members of a team.
// ListTeamMembers godoc
// @Summary      List team members
// @Tags         team
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Team ID"
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /teams/{id}/members [get]
func (h *TeamHandler) ListTeamMembers(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid team id"))
		return
	}

	orgID := middleware.GetOrgID(c)
	members, err := h.svc.ListTeamMembers(c.Request.Context(), orgID, teamID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondList(c, members)
}

// AddTeamMember adds a user to a team.
// AddTeamMember godoc
// @Summary      Add team member
// @Tags         team
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        id path string true "Team ID"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /teams/{id}/members [post]
func (h *TeamHandler) AddTeamMember(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid team id"))
		return
	}

	var input struct {
		UserID string `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	userID, err := uuid.Parse(input.UserID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid user id"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.AddTeamMember(c.Request.Context(), orgID, teamID, userID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, gin.H{"message": "team member added"})
}

// RemoveTeamMember removes a user from a team.
// RemoveTeamMember godoc
// @Summary      Remove team member
// @Tags         team
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Team ID"
// @Param        userId path string true "User ID"
// @Success      204
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /teams/{id}/members/{userId} [delete]
func (h *TeamHandler) RemoveTeamMember(c *gin.Context) {
	teamID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid team id"))
		return
	}
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail("invalid user id"))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.RemoveTeamMember(c.Request.Context(), orgID, teamID, userID); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondNoContent(c)
}

// GetInvitationByToken returns public info about an invitation (no auth required).
// GetInvitationByToken godoc
// @Summary      Get invitation info by token
// @Tags         team
// @Produce      json
// @Param        token query string true "Invitation token"
// @Success      200 {object} map[string]interface{}
// @Failure      404 {object} apidocs.ProblemDetail
// @Router       /team/invitations/info [get]
func (h *TeamHandler) GetInvitationByToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("token is required"))
		return
	}
	inv, err := h.svc.GetInvitationByToken(c.Request.Context(), token)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("invitation not found or expired"))
		return
	}
	httputil.RespondOK(c, gin.H{
		"email":      inv.Email,
		"role":       inv.Role,
		"expires_at": inv.ExpiresAt,
	})
}

// AcceptInvitationPublic allows accepting an invitation with registration (no prior auth).
// AcceptInvitationPublic godoc
// @Summary      Accept invitation with registration
// @Tags         team
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.AuthResult
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Router       /team/invitations/accept-public [post]
func (h *TeamHandler) AcceptInvitationPublic(c *gin.Context) {
	var input struct {
		Token       string `json:"token" binding:"required"`
		Password    string `json:"password" binding:"required,min=8"`
		DisplayName string `json:"display_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	result, err := h.svc.AcceptInvitationWithRegister(c.Request.Context(), input.Token, input.Password, input.DisplayName)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, result)
}
