package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

type teamStore struct {
	db bun.IDB
}

func (s *teamStore) Create(ctx context.Context, team *model.Team) error {
	_, err := s.db.NewInsert().Model(team).Returning("*").Exec(ctx)
	return err
}

func (s *teamStore) Update(ctx context.Context, team *model.Team) error {
	_, err := s.db.NewUpdate().Model(team).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *teamStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Team)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *teamStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Team, error) {
	team := new(model.Team)
	err := s.db.NewSelect().Model(team).Where("id = ?", id).Scan(ctx)
	return team, err
}

func (s *teamStore) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.Team, error) {
	var teams []model.Team
	err := s.db.NewSelect().
		Model(&teams).
		Where("org_id = ?", orgID).
		OrderExpr("name ASC").
		Scan(ctx)
	return teams, err
}

func (s *teamStore) CountProjects(ctx context.Context, teamID uuid.UUID) (int, error) {
	return s.db.NewSelect().
		Model((*model.Project)(nil)).
		Where("team_id = ?", teamID).
		Count(ctx)
}
