package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE workers ADD COLUMN IF NOT EXISTS build_command TEXT DEFAULT ''`,
			`ALTER TABLE workers ADD COLUMN IF NOT EXISTS deploy_command TEXT DEFAULT ''`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("worker build/deploy migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE workers DROP COLUMN IF EXISTS deploy_command`,
			`ALTER TABLE workers DROP COLUMN IF EXISTS build_command`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
