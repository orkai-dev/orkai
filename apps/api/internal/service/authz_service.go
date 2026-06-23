package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Authz centralizes project-scoped access decisions.
//
// Access rule: an admin may access any project in their org. A member may
// only access projects owned by a team they belong to.
type Authz struct {
	store store.Store
}

func NewAuthz(s store.Store) *Authz {
	return &Authz{store: s}
}

// isPrivileged reports whether the role has org-wide access (admin).
func isPrivileged(role string) bool {
	return role == string(model.RoleAdmin)
}

// AccessibleProjectIDs returns the project IDs a user may access. When isAll is
// true the user has org-wide access and the slice should be ignored (callers
// must not filter). For members, the returned slice is non-nil and contains the
// projects of every team they belong to (possibly empty).
func (a *Authz) AccessibleProjectIDs(ctx context.Context, userID uuid.UUID, role string, orgID uuid.UUID) (ids []uuid.UUID, isAll bool, err error) {
	if isPrivileged(role) {
		return nil, true, nil
	}

	teamIDs, err := a.store.TeamMembers().ListTeamIDsByUser(ctx, userID)
	if err != nil {
		return nil, false, err
	}

	// Always return a non-nil slice so callers can distinguish "no access" from
	// "org-wide access".
	if len(teamIDs) == 0 {
		return []uuid.UUID{}, false, nil
	}

	// Select only the project IDs (single indexed query) instead of hydrating
	// full project rows just to collect their IDs.
	projectIDs, err := a.store.Projects().ListIDsByTeams(ctx, orgID, teamIDs)
	if err != nil {
		return nil, false, err
	}
	return projectIDs, false, nil
}

// CanAccessProject reports whether the user may access the given project.
func (a *Authz) CanAccessProject(ctx context.Context, userID uuid.UUID, role string, orgID uuid.UUID, projectID uuid.UUID) (bool, error) {
	project, err := a.store.Projects().GetByID(ctx, projectID)
	if err != nil {
		return false, err
	}
	if project.OrgID != orgID {
		return false, nil
	}
	if isPrivileged(role) {
		return true, nil
	}

	teamIDs, err := a.store.TeamMembers().ListTeamIDsByUser(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, tid := range teamIDs {
		if tid == project.TeamID {
			return true, nil
		}
	}
	return false, nil
}
