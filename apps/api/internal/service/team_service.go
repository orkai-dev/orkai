package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type TeamService struct {
	store      store.Store
	jwtManager *auth.JWTManager
	logger     *slog.Logger
	notifSvc   *NotificationService
}

func NewTeamService(s store.Store, jwtManager *auth.JWTManager, logger *slog.Logger, notifSvc *NotificationService) *TeamService {
	return &TeamService{store: s, jwtManager: jwtManager, logger: logger, notifSvc: notifSvc}
}

// ============================================================================
// Members
// ============================================================================

func (s *TeamService) ListMembers(ctx context.Context, orgID uuid.UUID) ([]model.OrgMember, error) {
	users, _, err := s.store.Users().ListByOrg(ctx, orgID, store.ListParams{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, err
	}

	members := make([]model.OrgMember, len(users))
	for i, u := range users {
		members[i] = model.OrgMember{
			ID:          u.ID,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			FirstName:   u.FirstName,
			LastName:    u.LastName,
			AvatarURL:   u.AvatarURL,
			Role:        string(u.Role),
			CreatedAt:   u.CreatedAt,
		}
	}
	return members, nil
}

func (s *TeamService) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	if role != "admin" && role != "member" {
		return errors.New("role must be 'admin' or 'member'")
	}

	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.OrgID != orgID {
		return errors.New("user is not a member of this organization")
	}

	// Demoting the last admin would leave the org without any admin.
	if user.Role == model.RoleAdmin && role == string(model.RoleMember) {
		admins, err := s.store.Users().CountByRole(ctx, orgID, string(model.RoleAdmin))
		if err != nil {
			return err
		}
		if admins <= 1 {
			return errors.New("organization must have at least one admin")
		}
	}

	if err := s.store.Tx(ctx, func(ctx context.Context, tx store.Store) error {
		if err := tx.Users().UpdateRole(ctx, userID, role); err != nil {
			return err
		}
		return tx.APIKeys().RevokeAllForUser(ctx, userID)
	}); err != nil {
		return err
	}

	s.logger.Info("member role updated",
		slog.String("user_id", userID.String()),
		slog.String("role", role),
	)
	return nil
}

func (s *TeamService) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.OrgID != orgID {
		return errors.New("user is not a member of this organization")
	}

	// The org must always retain at least one admin.
	if user.Role == model.RoleAdmin {
		admins, err := s.store.Users().CountByRole(ctx, orgID, string(model.RoleAdmin))
		if err != nil {
			return err
		}
		if admins <= 1 {
			return errors.New("cannot remove the last admin")
		}
	}

	// Remove team memberships for this user
	memberships, err := s.store.TeamMembers().ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, m := range memberships {
		if err := s.store.TeamMembers().Remove(ctx, m.TeamID, userID); err != nil {
			s.logger.Warn("failed to remove team member during org removal",
				slog.String("team_id", m.TeamID.String()),
				slog.String("user_id", userID.String()),
				slog.Any("error", err),
			)
		}
	}

	// Remove user from the organization and revoke API keys atomically.
	if err := s.store.Tx(ctx, func(ctx context.Context, tx store.Store) error {
		if err := tx.Users().RemoveFromOrg(ctx, userID); err != nil {
			return err
		}
		return tx.APIKeys().RevokeAllForUser(ctx, userID)
	}); err != nil {
		return err
	}

	s.logger.Info("member removed from org",
		slog.String("user_id", userID.String()),
		slog.String("org_id", orgID.String()),
	)
	s.notifSvc.NotifyResourceDeleted(orgID, model.EventMemberRemoved,
		user.Email, fmt.Sprintf("Member %q was removed from the organization", user.Email))
	return nil
}

// ============================================================================
// Invitations
// ============================================================================

type InviteResult struct {
	Created    bool
	User       *model.User
	Invitation *model.Invitation
}

func (s *TeamService) InviteMember(ctx context.Context, orgID uuid.UUID, invitedBy uuid.UUID, email, role string) (*InviteResult, error) {
	if role != "admin" && role != "member" {
		return nil, errors.New("role must be 'admin' or 'member'")
	}

	email = strings.ToLower(strings.TrimSpace(email))

	// Check if user is already a member of the org
	existingUser, err := s.store.Users().GetByEmail(ctx, email)
	if err == nil {
		if existingUser.OrgID == orgID {
			return nil, errors.New("user is already a member of this organization")
		}
		return nil, errors.New("a user with this email already exists")
	}

	googleOnly, err := s.store.Settings().Get(ctx, model.SettingAuthGoogleOnly)
	if err != nil {
		return nil, err
	}
	if googleOnly == "true" {
		return s.inviteMemberGoogleOnly(ctx, orgID, email, role)
	}

	// Check for existing pending invitation
	existingInvs, _ := s.store.Invitations().ListByOrg(ctx, orgID)
	for _, inv := range existingInvs {
		if inv.Email == email && inv.AcceptedAt == nil && time.Now().Before(inv.ExpiresAt) {
			return nil, errors.New("an invitation is already pending for this email")
		}
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	inv := &model.Invitation{
		OrgID:     orgID,
		Email:     email,
		Role:      role,
		Token:     token,
		InvitedBy: &invitedBy,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
	}

	if err := s.store.Invitations().Create(ctx, inv); err != nil {
		return nil, err
	}

	s.logger.Info("invitation created",
		slog.String("email", email),
		slog.String("org_id", orgID.String()),
	)

	return &InviteResult{Invitation: inv}, nil
}

func (s *TeamService) inviteMemberGoogleOnly(ctx context.Context, orgID uuid.UUID, email, role string) (*InviteResult, error) {
	enabled, err := s.store.Settings().Get(ctx, model.SettingGoogleOAuthEnabled)
	if err != nil {
		return nil, err
	}
	if enabled != "true" {
		return nil, errors.New("Google OAuth must be enabled to invite users in Google-only mode")
	}
	clientID, err := s.store.Settings().Get(ctx, model.SettingGoogleOAuthClientID)
	if err != nil {
		return nil, err
	}
	clientSecret, err := s.store.Settings().Get(ctx, model.SettingGoogleOAuthClientSecret)
	if err != nil {
		return nil, err
	}
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("Google OAuth must be configured to invite users in Google-only mode")
	}

	allowedDomains, err := s.store.Settings().Get(ctx, model.SettingOAuthAllowedDomains)
	if err != nil {
		return nil, err
	}
	if !emailDomainAllowed(email, allowedDomains) {
		return nil, errors.New("email domain is not allowed")
	}

	displayName := email
	if parts := strings.Split(email, "@"); len(parts) > 0 && parts[0] != "" {
		displayName = parts[0]
	}

	user := &model.User{
		OrgID:        orgID,
		Email:        email,
		PasswordHash: "",
		DisplayName:  displayName,
		Role:         model.Role(role),
	}
	if err := s.store.Users().Create(ctx, user); err != nil {
		return nil, err
	}

	s.logger.Info("user created via google-only invite",
		slog.String("email", email),
		slog.String("org_id", orgID.String()),
	)

	return &InviteResult{Created: true, User: user}, nil
}

func (s *TeamService) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) error {
	inv, err := s.store.Invitations().GetByToken(ctx, token)
	if err != nil {
		return errors.New("invitation not found")
	}

	if time.Now().After(inv.ExpiresAt) {
		return errors.New("invitation has expired")
	}

	if inv.AcceptedAt != nil {
		return errors.New("invitation has already been accepted")
	}

	// Verify the accepting user's email matches the invitation
	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}
	if user.Email != inv.Email {
		return errors.New("invitation was sent to a different email address")
	}

	user.OrgID = inv.OrgID
	user.Role = model.Role(inv.Role)
	if err := s.store.Users().Update(ctx, user); err != nil {
		return err
	}

	// Mark invitation as accepted
	now := time.Now()
	inv.AcceptedAt = &now
	if err := s.store.Invitations().Update(ctx, inv); err != nil {
		return err
	}

	s.logger.Info("invitation accepted",
		slog.String("user_id", userID.String()),
		slog.String("org_id", inv.OrgID.String()),
	)
	return nil
}

func (s *TeamService) ListInvitations(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error) {
	return s.store.Invitations().ListByOrg(ctx, orgID)
}

func (s *TeamService) CancelInvitation(ctx context.Context, orgID, invID uuid.UUID) error {
	inv, err := s.store.Invitations().GetByID(ctx, invID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("invitation not found")
		}
		return err
	}
	if inv == nil || inv.OrgID != orgID {
		return errors.New("invitation not found")
	}

	if err := s.store.Invitations().Delete(ctx, invID); err != nil {
		return err
	}

	s.logger.Info("invitation cancelled", slog.String("invitation_id", invID.String()))
	s.notifSvc.NotifyResourceDeleted(orgID, model.EventInvitationCancelled,
		inv.Email, fmt.Sprintf("Invitation for %q was cancelled", inv.Email))
	return nil
}

// ============================================================================
// Teams
// ============================================================================

func (s *TeamService) CreateTeam(ctx context.Context, orgID uuid.UUID, name, description string) (*model.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("team name is required")
	}

	team := &model.Team{
		OrgID:       orgID,
		Name:        name,
		Description: strings.TrimSpace(description),
	}
	if err := s.store.Teams().Create(ctx, team); err != nil {
		return nil, err
	}

	s.logger.Info("team created",
		slog.String("team_id", team.ID.String()),
		slog.String("org_id", orgID.String()),
		slog.String("name", name),
	)
	return team, nil
}

func (s *TeamService) ListTeams(ctx context.Context, orgID uuid.UUID) ([]model.Team, error) {
	return s.store.Teams().ListByOrg(ctx, orgID)
}

func (s *TeamService) GetTeam(ctx context.Context, orgID, teamID uuid.UUID) (*model.Team, error) {
	team, err := s.store.Teams().GetByID(ctx, teamID)
	if err != nil {
		return nil, errors.New("team not found")
	}
	if team.OrgID != orgID {
		return nil, errors.New("team not found")
	}
	return team, nil
}

func (s *TeamService) DeleteTeam(ctx context.Context, orgID, teamID uuid.UUID) error {
	team, err := s.GetTeam(ctx, orgID, teamID)
	if err != nil {
		return err
	}

	count, err := s.store.Teams().CountProjects(ctx, team.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("cannot delete a team that still has projects; reassign or delete them first")
	}

	if err := s.store.Teams().Delete(ctx, team.ID); err != nil {
		return err
	}

	s.logger.Info("team deleted",
		slog.String("team_id", team.ID.String()),
		slog.String("org_id", orgID.String()),
	)
	s.notifSvc.NotifyResourceDeleted(orgID, model.EventTeamDeleted,
		team.Name, fmt.Sprintf("Team %q was deleted", team.Name))
	return nil
}

// ============================================================================
// Team Membership
// ============================================================================

func (s *TeamService) AddTeamMember(ctx context.Context, orgID, teamID, userID uuid.UUID) error {
	if _, err := s.GetTeam(ctx, orgID, teamID); err != nil {
		return err
	}

	user, err := s.store.Users().GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}
	if user.OrgID != orgID {
		return errors.New("user is not a member of this organization")
	}

	if err := s.store.TeamMembers().Add(ctx, teamID, userID); err != nil {
		return err
	}

	s.logger.Info("team member added",
		slog.String("team_id", teamID.String()),
		slog.String("user_id", userID.String()),
	)
	return nil
}

func (s *TeamService) RemoveTeamMember(ctx context.Context, orgID, teamID, userID uuid.UUID) error {
	team, err := s.GetTeam(ctx, orgID, teamID)
	if err != nil {
		return err
	}
	if err := s.store.TeamMembers().Remove(ctx, teamID, userID); err != nil {
		return err
	}

	s.logger.Info("team member removed",
		slog.String("team_id", teamID.String()),
		slog.String("user_id", userID.String()),
	)

	memberLabel := userID.String()
	if user, uerr := s.store.Users().GetByID(ctx, userID); uerr == nil && user != nil {
		memberLabel = user.Email
	}
	s.notifSvc.NotifyResourceDeleted(orgID, model.EventTeamMemberRemoved,
		memberLabel, fmt.Sprintf("Member %q was removed from team %q", memberLabel, team.Name))
	return nil
}

func (s *TeamService) ListTeamMembers(ctx context.Context, orgID, teamID uuid.UUID) ([]model.OrgMember, error) {
	if _, err := s.GetTeam(ctx, orgID, teamID); err != nil {
		return nil, err
	}
	return s.store.TeamMembers().ListUsersByTeam(ctx, teamID)
}

func (s *TeamService) ListTeamsForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return s.store.TeamMembers().ListTeamIDsByUser(ctx, userID)
}

func (s *TeamService) GetInvitationByToken(ctx context.Context, token string) (*model.Invitation, error) {
	inv, err := s.store.Invitations().GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, errors.New("invitation has expired")
	}
	if inv.AcceptedAt != nil {
		return nil, errors.New("invitation already accepted")
	}
	return inv, nil
}

func (s *TeamService) AcceptInvitationWithRegister(ctx context.Context, token, password, displayName string) (*AuthResult, error) {
	inv, err := s.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Check if user already exists
	_, userErr := s.store.Users().GetByEmail(ctx, inv.Email)
	if userErr == nil {
		return nil, errors.New("account already exists — log in first, then accept the invitation from your dashboard")
	}

	// Create the user
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		OrgID:        inv.OrgID,
		Email:        inv.Email,
		PasswordHash: string(hash),
		DisplayName:  displayName,
		Role:         model.Role(inv.Role),
	}
	if err := s.store.Users().Create(ctx, user); err != nil {
		return nil, err
	}

	// Mark invitation as accepted
	now := time.Now()
	inv.AcceptedAt = &now
	_ = s.store.Invitations().Update(ctx, inv)

	// Generate tokens
	tokens, err := s.jwtManager.GenerateTokenPair(user.ID, user.OrgID, string(user.Role), user.TokenVersion)
	if err != nil {
		return nil, err
	}

	s.logger.Info("invitation accepted via registration", slog.String("email", inv.Email))

	return &AuthResult{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
	}, nil
}
