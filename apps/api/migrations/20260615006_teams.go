package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			// Teams: a group of users within an organization.
			`CREATE TABLE IF NOT EXISTS teams (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				description TEXT DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				UNIQUE(org_id, name)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_teams_org_id ON teams(org_id)`,

			// Team membership (many-to-many users <-> teams).
			`CREATE TABLE IF NOT EXISTS team_members (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
				user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(team_id, user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_team_members_team ON team_members(team_id)`,
			`CREATE INDEX IF NOT EXISTS idx_team_members_user ON team_members(user_id)`,

			// Drop the unused project-level membership concept.
			`DROP TABLE IF EXISTS project_members CASCADE`,

			// Projects now belong to a team.
			`ALTER TABLE projects ADD COLUMN IF NOT EXISTS team_id UUID REFERENCES teams(id) ON DELETE RESTRICT`,
			`CREATE INDEX IF NOT EXISTS idx_projects_team_id ON projects(team_id)`,

			// Real-time UI updates for team changes.
			`CREATE OR REPLACE TRIGGER trigger_teams_updated_at
				BEFORE UPDATE ON teams
				FOR EACH ROW EXECUTE FUNCTION update_updated_at()`,
			`CREATE OR REPLACE TRIGGER trigger_teams_notify
				AFTER INSERT OR UPDATE OR DELETE ON teams
				FOR EACH ROW EXECUTE FUNCTION notify_change()`,
			`CREATE OR REPLACE TRIGGER trigger_team_members_notify
				AFTER INSERT OR UPDATE OR DELETE ON team_members
				FOR EACH ROW EXECUTE FUNCTION notify_change()`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("teams migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE projects DROP COLUMN IF EXISTS team_id`,
			`DROP TABLE IF EXISTS team_members CASCADE`,
			`DROP TABLE IF EXISTS teams CASCADE`,
			// Recreate the project_members table the down-migration removed.
			`CREATE TABLE IF NOT EXISTS project_members (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				role VARCHAR(20) NOT NULL DEFAULT 'viewer',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(project_id, user_id)
			)`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
