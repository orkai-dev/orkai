package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			// Project metadata (owner + environment)
			`ALTER TABLE projects ADD COLUMN IF NOT EXISTS owner VARCHAR(255) DEFAULT ''`,
			`ALTER TABLE projects ADD COLUMN IF NOT EXISTS environment VARCHAR(20) DEFAULT 'development'`,
			// Application metadata (is_critical)
			`ALTER TABLE applications ADD COLUMN IF NOT EXISTS is_critical BOOLEAN DEFAULT false`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("add metadata: %w", err)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE projects DROP COLUMN IF EXISTS owner`,
			`ALTER TABLE projects DROP COLUMN IF EXISTS environment`,
			`ALTER TABLE applications DROP COLUMN IF EXISTS is_critical`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
