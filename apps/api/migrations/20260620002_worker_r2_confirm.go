package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE workers ADD COLUMN IF NOT EXISTS r2_confirmed_buckets JSONB NOT NULL DEFAULT '[]'`,
			`ALTER TABLE worker_deployments ADD COLUMN IF NOT EXISTS r2_pending JSONB NOT NULL DEFAULT '[]'`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("worker r2 confirm migration: %w\nquery: %s", err, q)
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		stmts := []string{
			`ALTER TABLE worker_deployments DROP COLUMN IF EXISTS r2_pending`,
			`ALTER TABLE workers DROP COLUMN IF EXISTS r2_confirmed_buckets`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
