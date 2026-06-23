package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS build_enabled BOOLEAN DEFAULT false`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS package_manager VARCHAR(20) DEFAULT 'auto'`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS install_command TEXT DEFAULT ''`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS build_command TEXT DEFAULT ''`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS output_dir VARCHAR(255) DEFAULT ''`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS root_directory VARCHAR(255) DEFAULT '.'`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS node_version VARCHAR(20) DEFAULT ''`,
			`ALTER TABLE pages ADD COLUMN IF NOT EXISTS build_env_vars JSONB DEFAULT '{}'`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("pages build migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE pages DROP COLUMN IF EXISTS build_env_vars`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS node_version`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS root_directory`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS output_dir`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS build_command`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS install_command`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS package_manager`,
			`ALTER TABLE pages DROP COLUMN IF EXISTS build_enabled`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
