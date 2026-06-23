package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

type teamMemberStore struct {
	db bun.IDB
}

func (s *teamMemberStore) Add(ctx context.Context, teamID, userID uuid.UUID) error {
	tm := &model.TeamMember{TeamID: teamID, UserID: userID}
	_, err := s.db.NewInsert().
		Model(tm).
		On("CONFLICT (team_id, user_id) DO NOTHING").
		Exec(ctx)
	return err
}

func (s *teamMemberStore) Remove(ctx context.Context, teamID, userID uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*model.TeamMember)(nil)).
		Where("team_id = ?", teamID).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}

func (s *teamMemberStore) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]model.TeamMember, error) {
	var members []model.TeamMember
	err := s.db.NewSelect().
		Model(&members).
		Where("team_id = ?", teamID).
		OrderExpr("created_at ASC").
		Scan(ctx)
	return members, err
}

func (s *teamMemberStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.TeamMember, error) {
	var members []model.TeamMember
	err := s.db.NewSelect().
		Model(&members).
		Where("user_id = ?", userID).
		OrderExpr("created_at ASC").
		Scan(ctx)
	return members, err
}

func (s *teamMemberStore) ListTeamIDsByUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := s.db.NewSelect().
		Model((*model.TeamMember)(nil)).
		Column("team_id").
		Where("user_id = ?", userID).
		Scan(ctx, &ids)
	return ids, err
}

func (s *teamMemberStore) ListUsersByTeam(ctx context.Context, teamID uuid.UUID) ([]model.OrgMember, error) {
	var members []model.OrgMember
	err := s.db.NewSelect().
		ColumnExpr("u.id, u.email, u.display_name, u.first_name, u.last_name, u.avatar_url, u.role, tm.created_at").
		TableExpr("team_members AS tm").
		Join("JOIN users AS u ON u.id = tm.user_id").
		Where("tm.team_id = ?", teamID).
		Where("u.deleted_at IS NULL").
		OrderExpr("tm.created_at ASC").
		Scan(ctx, &members)
	return members, err
}
