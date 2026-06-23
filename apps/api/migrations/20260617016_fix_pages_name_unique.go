package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// pages uses BaseModel soft delete (deleted_at). The table-level UNIQUE
		// (project_id, name) blocked reusing a name after delete because the row
		// remained. Match the domains fix: partial index on active rows only.
		_, _ = db.ExecContext(ctx, `ALTER TABLE pages DROP CONSTRAINT IF EXISTS pages_project_id_name_key`)
		_, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_pages_project_id_name_active ON pages(project_id, name) WHERE deleted_at IS NULL`)
		if err != nil {
			return fmt.Errorf("create pages partial unique index: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, _ = db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_pages_project_id_name_active`)
		_, err := db.ExecContext(ctx, `ALTER TABLE pages ADD CONSTRAINT pages_project_id_name_key UNIQUE (project_id, name)`)
		return err
	})
}
