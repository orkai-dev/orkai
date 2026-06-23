package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type projectStore struct {
	db bun.IDB
}

// GetByID loads a project by primary key and attaches its team.
// Tenancy (org_id) is not checked here; HTTP handlers enforce it via Authz.CanAccessProject.
func (s *projectStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	project := new(model.Project)
	err := s.db.NewSelect().Model(project).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	projects := []model.Project{*project}
	if err := attachTeams(ctx, s.db, projects); err != nil {
		return nil, err
	}
	return &projects[0], nil
}

func (s *projectStore) Create(ctx context.Context, project *model.Project) error {
	_, err := s.db.NewInsert().Model(project).Returning("*").Exec(ctx)
	return err
}

func (s *projectStore) Update(ctx context.Context, project *model.Project) error {
	_, err := s.db.NewUpdate().Model(project).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *projectStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Project)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *projectStore) ListByOrg(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
	var projects []model.Project
	count, err := s.db.NewSelect().
		Model(&projects).
		Where("org_id = ?", orgID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := attachTeams(ctx, s.db, projects); err != nil {
		return nil, 0, err
	}
	return projects, count, nil
}

func (s *projectStore) ListIDsByTeams(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if len(teamIDs) == 0 {
		return ids, nil
	}
	err := s.db.NewSelect().
		Model((*model.Project)(nil)).
		Column("id").
		Where("org_id = ?", orgID).
		Where("team_id IN (?)", bun.List(teamIDs)).
		Scan(ctx, &ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *projectStore) ListByTeams(ctx context.Context, orgID uuid.UUID, teamIDs []uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
	var projects []model.Project
	if len(teamIDs) == 0 {
		return projects, 0, nil
	}
	count, err := s.db.NewSelect().
		Model(&projects).
		Where("org_id = ?", orgID).
		Where("team_id IN (?)", bun.List(teamIDs)).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := attachTeams(ctx, s.db, projects); err != nil {
		return nil, 0, err
	}
	return projects, count, nil
}

func attachTeams(ctx context.Context, db bun.IDB, projects []model.Project) error {
	if len(projects) == 0 {
		return nil
	}

	teamIDs := make([]uuid.UUID, 0, len(projects))
	seen := make(map[uuid.UUID]struct{}, len(projects))
	for _, project := range projects {
		if project.TeamID == uuid.Nil {
			continue
		}
		if _, ok := seen[project.TeamID]; ok {
			continue
		}
		seen[project.TeamID] = struct{}{}
		teamIDs = append(teamIDs, project.TeamID)
	}
	if len(teamIDs) == 0 {
		return nil
	}

	var teams []model.Team
	if err := db.NewSelect().
		Model(&teams).
		Where("id IN (?)", bun.List(teamIDs)).
		Scan(ctx); err != nil {
		return err
	}

	byID := make(map[uuid.UUID]*model.Team, len(teams))
	for i := range teams {
		byID[teams[i].ID] = &teams[i]
	}
	for i := range projects {
		if projects[i].TeamID == uuid.Nil {
			continue
		}
		team := byID[projects[i].TeamID]
		if team == nil {
			return fmt.Errorf("team %s not found for project %s", projects[i].TeamID, projects[i].ID)
		}
		projects[i].Team = team
	}
	return nil
}
