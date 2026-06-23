package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS is_super_admin BOOLEAN NOT NULL DEFAULT false`)
		if err != nil {
			return fmt.Errorf("add users.is_super_admin: %w", err)
		}
		_, err = db.ExecContext(ctx, `
			UPDATE users SET is_super_admin = true
			WHERE id = (
				SELECT id FROM users
				WHERE role = 'admin' AND deleted_at IS NULL
				ORDER BY created_at ASC
				LIMIT 1
			)
		`)
		if err != nil {
			return fmt.Errorf("backfill users.is_super_admin: %w", err)
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `ALTER TABLE users DROP COLUMN IF EXISTS is_super_admin`)
		if err != nil {
			return fmt.Errorf("drop users.is_super_admin: %w", err)
		}
		return nil
	})
}
